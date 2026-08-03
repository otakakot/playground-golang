package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// 実際に起動した PostgreSQL / Redis に接続して
// NotFound エラーを発生させ、OSS/ライブラリの実装パターンでハンドリングする。
//
// 調査対象の実装パターン:
//   エラー型を使う表現
//   - センチネルエラー (database/sql.ErrNoRows, gorm.ErrRecordNotFound, mongo.ErrNoDocuments, redis.Nil, io/fs.ErrNotExist)
//   - 型付きエラー (k8s apimachinery の StatusError / ent の NotFoundError)
//   - ErrorCode() / ErrorFault() 付きの型付きエラー (AWS SDK v2 / smithy)
//   - Is メソッドでのエラーコード→ドメイン概念のマッピング (syscall.Errno.Is)
//   - MaskNotFound によるマスク (ent)
//   エラー型を使わない表現
//   - comma ok (os.LookupEnv / sync.Map.Load / samber/lo.Find)
//   - Option 型 (samber/mo)
//   - 存在確認 API の分離 (viper.IsSet)
//   - 空コレクション (mongo Find)
//   - 番兵値 (strings.Index の -1 / redis Exists の 0)
//   - Null 型 (sql.NullString)
//   - http.NotFound helper
//   インターフェース層 (NotFound の表現を実装側に押し付ける)
//   - センチネル契約 (ErrNotFound を %w でラップして返す)
//   - 型付きエラー契約 (NotFoundError を返す)
//   - 自己申告契約 (NotFound() bool を実装した自前のエラー型)
//   - Exists / Lookup によるシグネチャ強制
//   - conformance 契約テスト

// --- ドメイン層 ------------------------------------------------------------------

// センチネルエラー (database/sql.ErrNoRows, gorm.ErrRecordNotFound と同じ考え方)
var ErrNotFound = errors.New("not found")

// 型付きエラー (k8s.io/apimachinery の StatusError / NewNotFound 風)。
// リソース種別と名前のメタデータを持つ。
type NotFoundError struct {
	Resource string // 例: users
	Name     string // 例: user id
}

func NewNotFoundError(resource, name string) *NotFoundError {
	return &NotFoundError{
		Resource: resource,
		Name:     name,
	}
}

func (err *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", err.Resource, err.Name)
}

// ErrorCode / ErrorFault は AWS SDK v2 (smithy) のエラー型と同じ API。
// ErrorCode はエラーコード文字列、ErrorFault はクライアント/サーバー起因の分類を返す。
func (err *NotFoundError) ErrorCode() string {
	return "NotFoundError"
}

type ErrorFault int

const (
	FaultUnknown ErrorFault = iota
	FaultClient
	FaultServer
)

func (err *NotFoundError) ErrorFault() ErrorFault {
	return FaultClient
}

// Is メソッドの実装。ent の NotFoundError と同様にリソース単位で比較する。
// センチネル ErrNotFound とも等価とみなすことで、errors.Is(err, ErrNotFound) でも判定できる。
func (err *NotFoundError) Is(target error) bool {
	if target == ErrNotFound {
		return true
	}

	t, ok := target.(*NotFoundError)
	if !ok {
		return false
	}

	return err.Resource == t.Resource
}

// NotFoundReporter は実装側が自前の NotFound 型を定義する場合の契約 (方法3: 自己申告)。
// エラー型に NotFound() bool を実装すれば、IsNotFound で判定できる。
type NotFoundReporter interface {
	error
	NotFound() bool
}

// NotFoundError に自己申告を実装する。
func (err *NotFoundError) NotFound() bool {
	return true
}

// IsNotFound は3系統の契約を横断して判定する。
func IsNotFound(err error) bool {
	// 契約1: センチネル (ErrNotFound を %w でラップして返す)。
	// NotFoundError は Is メソッドで ErrNotFound と等価とみなすので、こちらにも合致する。
	if errors.Is(err, ErrNotFound) {
		return true
	}

	// 契約2: 型付きエラー (NotFoundError を返す)
	var nf *NotFoundError

	if errors.As(err, &nf) {
		return true
	}

	// 契約3: 自己申告 (自前の NotFound 型に NotFound() bool を実装)
	var reporter NotFoundReporter

	if errors.As(err, &reporter) && reporter.NotFound() {
		return true
	}

	return false
}

// MaskNotFound は ent の MaskNotFound と同じ。NotFound エラーなら nil にマスクして
// 「無かったものとして扱う」。存在チェックをエラー処理にしたくない場合に使う。
func MaskNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}

	return err
}

// MapPQError は syscall.Errno.Is と同じ思想。
// PostgreSQL の SQLSTATE をドメインエラーへマッピングする。
// (Errno.Is は ENOENT を fs.ErrNotExist にマッピングしている)
func MapPQError(err error) error {
	var pqErr *pq.Error

	if !errors.As(err, &pqErr) {
		return err
	}

	switch pqErr.SQLState() {
	case "42P01": // undefined_table: テーブルが存在しない
		return fmt.Errorf("pq %s: %w", pqErr.SQLState(), NewNotFoundError("table", pqErr.Table))
	default:
		return err
	}
}

// NotFound 系エラーを HTTP 404 に変換する。
// 自前の契約 (センチネル/型付き/自己申告) に加え、
// 素のドライバ/クライアント/OS のセンチネルも扱える。
func ToHTTPStatus(err error) int {
	switch {
	case IsNotFound(err), // 自前の契約
		errors.Is(err, sql.ErrNoRows), // 素のドライバ/クライアント/OS のセンチネル
		errors.Is(err, redis.Nil),
		errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// --- Option 型 (samber/mo の Option[T] と同じ API) ---------------------------------

// Option は存在/不在を型で表現するコンテナ。エラー型を使わずに NotFound を None で表現する。
type Option[T any] struct {
	isPresent bool
	value     T
}

func Some[T any](value T) Option[T] {
	return Option[T]{isPresent: true, value: value}
}

func None[T any]() Option[T] {
	return Option[T]{isPresent: false}
}

// TupleToOption は comma ok の (value, ok) を Option に変換する (mo と同じ)。
func TupleToOption[T any](value T, ok bool) Option[T] {
	if ok {
		return Some(value)
	}

	return None[T]()
}

// PointerToOption は nil ポインタを None に変換する (mo と同じ)。
func PointerToOption[T any](value *T) Option[T] {
	if value == nil {
		return None[T]()
	}

	return Some(*value)
}

func (o Option[T]) IsSome() bool {
	return o.isPresent
}

func (o Option[T]) IsNone() bool {
	return !o.isPresent
}

func (o Option[T]) Get() (T, bool) {
	return o.value, o.isPresent
}

// --- リポジトリ層 (PostgreSQL) ------------------------------------------------------

type User struct {
	ID       string
	Name     string
	Nickname sql.NullString
}

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

// queryUser は取得の共通処理。NotFound は sql.ErrNoRows のまま返す。
func (r *UserRepo) queryUser(ctx context.Context, id string) (User, error) {
	var u User

	err := r.db.QueryRowContext(ctx, "SELECT id, name, nickname FROM users WHERE id = $1", id).
		Scan(&u.ID, &u.Name, &u.Nickname)

	return u, err
}

// GetByID はエラー型を使うパターン (gorm / database/sql 流儀)。
// ドライバ固有のセンチネル(sql.ErrNoRows)をドメインの型付きエラーへ変換する
// (gorm が ErrRecordNotFound に変換するのと同じ思想)。
func (r *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
	u, err := r.queryUser(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("users repo: %w", NewNotFoundError("users", id))
	}
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// LookupByID はエラー型を使わない comma ok パターン (os.LookupEnv / sync.Map.Load / samber/lo.Find 流儀)。
// NotFound を false で表現する。
// 注意: NotFound 以外のエラーも false として握り潰す。エラー判定が必要な場合は GetByID を使う。
func (r *UserRepo) LookupByID(ctx context.Context, id string) (*User, bool) {
	u, err := r.queryUser(ctx, id)
	if err != nil {
		return nil, false
	}

	return &u, true
}

// FindByID はエラー型を使わない Option 型パターン (samber/mo 流儀)。
// NotFound を None で表現する。
func (r *UserRepo) FindByID(ctx context.Context, id string) Option[User] {
	u, err := r.queryUser(ctx, id)
	if err != nil {
		return None[User]()
	}

	return Some(u)
}

// Exists は存在確認 API を分離するパターン (viper.IsSet 流儀)。
// 取得はエラーで表現するが、存在確認は bool で表現する。
func (r *UserRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool

	err := r.db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)", id).Scan(&exists)

	return exists, err
}

// List は 0 件でもエラーにせず空コレクションを返すパターン (mongo の Find / 一覧取得 API の流儀)。
func (r *UserRepo) List(ctx context.Context) ([]User, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, nickname FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]User, 0)

	for rows.Next() {
		var u User

		if err := rows.Scan(&u.ID, &u.Name, &u.Nickname); err != nil {
			return nil, err
		}

		users = append(users, u)
	}

	return users, rows.Err()
}

// --- キャッシュ層 (Redis) -----------------------------------------------------------

type Cache struct {
	rdb *redis.Client
}

func NewCache(rdb *redis.Client) *Cache {
	return &Cache{rdb: rdb}
}

func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		// クライアント固有のセンチネル(redis.Nil)をドメインの型付きエラーへ変換する
		return "", fmt.Errorf("cache repo: %w", NewNotFoundError("cache", key))
	}
	if err != nil {
		return "", err
	}

	return val, nil
}

// --- ハンドラー層 --------------------------------------------------------------------

func handleGetUser(ctx context.Context, repo *UserRepo, w http.ResponseWriter, id string) {
	user, err := repo.GetByID(ctx, id)
	if err != nil {
		w.WriteHeader(ToHTTPStatus(err))
		fmt.Fprintf(w, "%s\n", err)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%+v\n", user)
}

func handleGetCache(ctx context.Context, cache *Cache, w http.ResponseWriter, key string) {
	val, err := cache.Get(ctx, key)
	if err != nil {
		w.WriteHeader(ToHTTPStatus(err))
		fmt.Fprintf(w, "%s\n", err)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s\n", val)
}

// --- インターフェース層: NotFound の表現を実装側に押し付ける ----------------------------

// UserStore は契約。実装側に「NotFound の表現」と「存在判定」を委ねる。
type UserStore interface {
	// Get は見つからない場合に NotFound 系エラーを返す契約。
	// 表現(センチネル/型付き/自己申告)は実装側が選ぶ。
	Get(ctx context.Context, id string) (*User, error)
	// Exists は存在確認。実装側が「あるかどうか」の判定を担う。
	Exists(ctx context.Context, id string) (bool, error)
	// Lookup は comma ok で存在/不在を表現する契約。シグネチャで表現を強制する。
	// エラーはそのまま伝搬し、NotFound だけを false で表現する。
	Lookup(ctx context.Context, id string) (*User, bool, error)
}

// --- 実装A: センチネル契約 (方法1) ------------------------------------------------------

// RedisUserStore は ErrNotFound を %w でラップして返す契約。文脈付与は実装側の裁量。
type RedisUserStore struct {
	rdb *redis.Client
}

func NewRedisUserStore(rdb *redis.Client) *RedisUserStore {
	return &RedisUserStore{rdb: rdb}
}

func (s *RedisUserStore) Get(ctx context.Context, id string) (*User, error) {
	val, err := s.rdb.Get(ctx, "user:"+id).Result()
	if errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("redis store: %w", ErrNotFound)
	}
	if err != nil {
		return nil, err
	}

	return &User{ID: id, Name: val}, nil
}

func (s *RedisUserStore) Exists(ctx context.Context, id string) (bool, error) {
	n, err := s.rdb.Exists(ctx, "user:"+id).Result()

	return n == 1, err
}

func (s *RedisUserStore) Lookup(ctx context.Context, id string) (*User, bool, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return u, true, nil
}

// --- 実装B: 型付きエラー契約 (方法2) ------------------------------------------------------

// PostgresUserStore は NotFoundError(型付き)を返す契約。メタデータ(どのリソースが無いか)も実装側が決める。
type PostgresUserStore struct {
	repo *UserRepo
}

func NewPostgresUserStore(repo *UserRepo) *PostgresUserStore {
	return &PostgresUserStore{repo: repo}
}

func (s *PostgresUserStore) Get(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PostgresUserStore) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

func (s *PostgresUserStore) Lookup(ctx context.Context, id string) (*User, bool, error) {
	// comma ok ではエラーを握り潰せないので、GetByID を使って NotFound だけを false に変換する。
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return u, true, nil
}

// --- 実装C: 自己申告契約 (方法3) -----------------------------------------------------------

// ArticleStore は契約。Get は自前の NotFound 型で NotFound を申告する。
type ArticleStore interface {
	Get(ctx context.Context, id string) (*Article, error)
	Exists(ctx context.Context, id string) (bool, error)
	Lookup(ctx context.Context, id string) (*Article, bool, error)
}

type Article struct {
	ID    string
	Title string
}

// articleNotFoundError は実装側が定義した自前の NotFound 型。NotFound() で自己申告する。
type articleNotFoundError struct {
	ID string
}

func (err *articleNotFoundError) Error() string {
	return fmt.Sprintf("article %q not found", err.ID)
}

func (err *articleNotFoundError) NotFound() bool {
	return true
}

// MemoryArticleStore は記事のインメモリ実装。実装の差し替えが自由なことを示す。
type MemoryArticleStore struct {
	articles map[string]Article
}

func NewMemoryArticleStore() *MemoryArticleStore {
	return &MemoryArticleStore{articles: make(map[string]Article)}
}

func (s *MemoryArticleStore) Get(ctx context.Context, id string) (*Article, error) {
	a, ok := s.articles[id]
	if !ok {
		return nil, fmt.Errorf("article store: %w", &articleNotFoundError{ID: id})
	}

	return &a, nil
}

func (s *MemoryArticleStore) Exists(ctx context.Context, id string) (bool, error) {
	_, ok := s.articles[id]

	return ok, nil
}

func (s *MemoryArticleStore) Lookup(ctx context.Context, id string) (*Article, bool, error) {
	a, ok := s.articles[id]

	return &a, ok, nil
}

// --- 方法6: conformance 契約テスト ----------------------------------------------------------

// AssertUserStore は実装が契約を守っているかを検証する conformance チェック。
// 実運用では _test.go に置き、全ての実装を同一テストで検証する。
func AssertUserStore(store UserStore, missingID string) error {
	ctx := context.Background()

	// 存在しない ID に対して必ず NotFound が判定できること
	if _, err := store.Get(ctx, missingID); !IsNotFound(err) {
		return fmt.Errorf("Get must return NotFound, got %v", err)
	}

	// 存在確認は false であること
	if exists, err := store.Exists(ctx, missingID); err != nil || exists {
		return fmt.Errorf("Exists must be (false, nil), got (%v, %v)", exists, err)
	}

	// comma ok は (nil, false, nil) であること。NotFound 以外のエラーは伝搬する。
	u, ok, err := store.Lookup(ctx, missingID)
	if err != nil || ok {
		return fmt.Errorf("Lookup must be (nil, false, nil), got (%v, %v, %v)", u, ok, err)
	}

	return nil
}

// --- ハンドラー層 (インターフェース版) -------------------------------------------------------

func handleGetFromStore(ctx context.Context, store UserStore, w http.ResponseWriter, id string) {
	user, err := store.Get(ctx, id)
	if err != nil {
		w.WriteHeader(ToHTTPStatus(err))
		fmt.Fprintf(w, "%s\n", err)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%+v\n", user)
}

func handleGetArticle(ctx context.Context, store ArticleStore, w http.ResponseWriter, id string) {
	article, err := store.Get(ctx, id)
	if err != nil {
		w.WriteHeader(ToHTTPStatus(err))
		fmt.Fprintf(w, "%s\n", err)

		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%+v\n", article)
}

// --- main ---------------------------------------------------------------------------

type deps struct {
	db    *sql.DB
	rdb   *redis.Client
	repo  *UserRepo
	cache *Cache
}

func main() {
	ctx := context.Background()

	db := openPostgres(ctx)
	defer db.Close()

	if _, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS users;
		CREATE TABLE users (
			id       TEXT PRIMARY KEY,
			name     TEXT NOT NULL,
			nickname TEXT
		);
	`); err != nil {
		panic(err)
	}

	rdb := openRedis()
	defer rdb.Close()

	deps := &deps{
		db:    db,
		rdb:   rdb,
		repo:  NewUserRepo(db),
		cache: NewCache(rdb),
	}

	missingID := uuid.NewString()
	missingKey := "user:" + uuid.NewString()

	demoErrorPatterns(ctx, deps, missingID, missingKey)
	demoNoErrorPatterns(ctx, deps, missingID, missingKey)
	demoInterfacePatterns(ctx, deps, missingID)
}

func openPostgres(ctx context.Context) *sql.DB {
	postgresPort := os.Getenv("POSTGRES_PORT")
	if postgresPort == "" {
		postgresPort = "5432"
	}

	dsn := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", postgresPort)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}

	if err := db.PingContext(ctx); err != nil {
		panic(err)
	}

	return db
}

func openRedis() *redis.Client {
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("localhost:%s", redisPort),
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}

	return rdb
}

func demoErrorPatterns(ctx context.Context, d *deps, missingID, missingKey string) {
	{
		// センチネル→型付きエラー変換: 存在しないユーザー。
		// 内部では sql.ErrNoRows が発生し、NotFoundError に変換されたのち 404 になる。
		rec := httptest.NewRecorder()
		handleGetUser(ctx, d.repo, rec, missingID)
		fmt.Printf("GET user %s => %d (%s)\n", missingID, rec.Code, rec.Body.String())
	}

	{
		// センチネル→型付きエラー変換: 存在しないキャッシュキー。
		// 内部では redis.Nil が発生し、404 になる。
		rec := httptest.NewRecorder()
		handleGetCache(ctx, d.cache, rec, missingKey)
		fmt.Printf("GET cache %s => %d (%s)\n", missingKey, rec.Code, rec.Body.String())
	}

	{
		// AWS SDK v2 風: ErrorCode / ErrorFault を持つ型付きエラー
		notFound := NewNotFoundError("users", missingID)
		fmt.Printf("ErrorCode=%s ErrorFault=%d\n", notFound.ErrorCode(), notFound.ErrorFault())
	}

	{
		// ent 風: MaskNotFound は NotFound を nil にマスクする
		masked := MaskNotFound(fmt.Errorf("wrapped: %w", NewNotFoundError("users", missingID)))
		fmt.Printf("MaskNotFound => %v\n", masked == nil)
	}

	{
		// syscall.Errno.Is 風: OS エラーコード(ENOENT)が fs.ErrNotExist にマッピングされる
		// (os.PathError の Unwrap と Errno.Is の組み合わせで errors.Is が成立する)
		_, fsErr := os.Open("/no/such/file")
		fmt.Printf("fs.ErrNotExist => %v (HTTP %d)\n", errors.Is(fsErr, fs.ErrNotExist), ToHTTPStatus(fsErr))
	}

	{
		// syscall.Errno.Is 風: PostgreSQL の SQLSTATE(42P01: undefined_table)を
		// NotFoundError にマッピングする
		var x string

		pqErr := dbQueryErr(ctx, d.db, "SELECT * FROM no_such_table", &x)
		mapped := MapPQError(pqErr)
		fmt.Printf("MapPQError => %v (HTTP %d)\n", mapped, ToHTTPStatus(mapped))
	}
}

func demoNoErrorPatterns(ctx context.Context, d *deps, missingID, missingKey string) {
	{
		// comma ok (os.LookupEnv / samber/lo.Find 流儀)
		user, ok := d.repo.LookupByID(ctx, missingID)
		fmt.Printf("LookupByID => ok=%v user=%v\n", ok, user)
	}

	{
		// Option 型 (samber/mo 流儀)。NotFound を None で表現する。
		user := d.repo.FindByID(ctx, missingID)
		val, present := user.Get()
		fmt.Printf("FindByID => IsNone=%v Get=(%v, %v)\n", user.IsNone(), val, present)
	}

	{
		// Option の変換ヘルパー (TupleToOption / PointerToOption)
		tupleVal, tupleOK := TupleToOption("v", true).Get()
		fmt.Printf("TupleToOption => (%v, %v)\n", tupleVal, tupleOK)
		fmt.Printf("PointerToOption(nil) => IsNone=%v\n", PointerToOption[string](nil).IsNone())
	}

	{
		// 存在確認 API の分離 (viper.IsSet 流儀)
		exists, _ := d.repo.Exists(ctx, missingID)
		fmt.Printf("Exists => %v\n", exists)
	}

	{
		// 0 件なら空コレクション (mongo Find 流儀)
		users, _ := d.repo.List(ctx)
		fmt.Printf("List => %d items\n", len(users))
	}

	{
		// 番兵値 (strings.Index の -1 / redis Exists の 0)
		fmt.Printf("Index => %d\n", strings.Index("a,b,c", "z"))

		n, _ := d.rdb.Exists(ctx, missingKey).Result()
		fmt.Printf("redis Exists => %d\n", n)
	}

	{
		// Null 型 (sql.NullString 等。JSON の null と同じ表現)。
		// 実データの NULL カラムで検証する。
		if _, err := d.db.ExecContext(ctx, "INSERT INTO users (id, name, nickname) VALUES ($1, $2, $3)",
			uuid.NewString(), "no-nickname", nil); err != nil {
			panic(err)
		}

		var u User

		if err := d.db.QueryRowContext(ctx, "SELECT id, name, nickname FROM users WHERE nickname IS NULL").
			Scan(&u.ID, &u.Name, &u.Nickname); err != nil {
			panic(err)
		}

		fmt.Printf("NullString => Valid=%v Value=%q\n", u.Nickname.Valid, u.Nickname.String)
	}

	{
		// HTTP 層はエラーを返さず 404 を書く (http.NotFound helper)
		rec := httptest.NewRecorder()
		http.NotFound(rec, httptest.NewRequest(http.MethodGet, "/users/"+missingID, nil))
		fmt.Printf("http.NotFound => %d (%s)\n", rec.Code, rec.Body.String())
	}
}

// dbQueryErr は Scan の結果をそのまま返すだけのヘルパー。デモの表示を短くするためのもの。
func dbQueryErr(ctx context.Context, db *sql.DB, query string, dest any) error {
	return db.QueryRowContext(ctx, query).Scan(dest)
}

func demoInterfacePatterns(ctx context.Context, d *deps, missingID string) {
	// 同じ UserStore 契約に対して実装を差し替える。呼び出し側は実装を知らなくていい。
	stores := []struct {
		name  string
		store UserStore
	}{
		{"postgres (型付きエラー契約)", NewPostgresUserStore(d.repo)},
		{"redis    (センチネル契約)", NewRedisUserStore(d.rdb)},
	}

	for _, s := range stores {
		rec := httptest.NewRecorder()
		handleGetFromStore(ctx, s.store, rec, missingID)
		fmt.Printf("%s => %d (%s)\n", s.name, rec.Code, rec.Body.String())

		// 契約が守られているかは conformance チェックで担保する (方法6)
		if err := AssertUserStore(s.store, missingID); err != nil {
			panic(err)
		}
		fmt.Printf("  contract ok\n")
	}

	// 自前の NotFound 型で自己申告する実装 (方法3)
	articleStore := NewMemoryArticleStore()

	rec := httptest.NewRecorder()
	handleGetArticle(ctx, articleStore, rec, missingID)
	fmt.Printf("memory   (自己申告契約) => %d (%s)\n", rec.Code, rec.Body.String())
}
