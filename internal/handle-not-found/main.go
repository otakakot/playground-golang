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

// 実際に起動した PostgreSQL / Redis に接続し、
// OSS や標準ライブラリの実装パターンをそのまま用いて NotFound を扱う。
//
// 各パターンの典拠:
//
//  1. センチネルエラー
//     標準: database/sql.ErrNoRows, os.ErrNotExist, io/fs.ErrNotExist, io.EOF
//     OSS : gorm.ErrRecordNotFound, mongo.ErrNoDocuments, redis.Nil
//
//  2. 型付きエラー
//     OSS : ent.NotFoundError, k8s.io/apimachinery/pkg/api/errors.StatusError
//
//  3. errors.Is 拡張（Is メソッド）
//     標準: syscall.Errno.Is, os.ErrNotExist
//
//  4. エラーコード / Fault 分類
//     OSS : AWS SDK for Go v2 (smithy) ErrorCode / ErrorFault
//
//  5. 自己申告型エラー
//     慣例: エラー型が NotFound() bool を実装
//
//  6. MaskNotFound
//     OSS : ent.MaskNotFound
//
//  7. 低レイヤーエラーコード → ドメインエラー
//     標準: syscall.Errno.Is が ENOENT を fs.ErrNotExist にマッピング
//     OSS : lib/pq の SQLState をドメインエラーに変換
//
//  8. エラー型を使わない表現
//     標準: os.LookupEnv, sync.Map.Load, strings.Index, sql.NullString, http.NotFound
//     OSS : redis.Exists, samber/mo.Option
//
//  9. インターフェース層での契約
//     標準: database/sql driver tests での conformance test 思想
//     OSS : ent/gorm での「Get は NotFoundError を返す」契約

// ============================================================
// 1. センチネルエラー
// ============================================================
//
// database/sql.ErrNoRows や os.ErrNotExist、gorm.ErrRecordNotFound、
// mongo.ErrNoDocuments、redis.Nil と同じく、パッケージレベルの値で表す。

var ErrNotFound = errors.New("not found")

// ============================================================
// 2. 型付きエラー
// ============================================================
//
// ent.NotFoundError や k8s apimachinery.StatusError と同じく、
// リソース種別・名前などのメタデータを持つ。

type NotFoundError struct {
	Resource string
	Name     string
}

func NewNotFoundError(resource, name string) *NotFoundError {
	return &NotFoundError{Resource: resource, Name: name}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.Name)
}

// ============================================================
// 3. errors.Is 拡張
// ============================================================
//
// syscall.Errno.Is と同じ思想。型付きエラーが「センチネル ErrNotFound と等価」と
// 宣言することで、呼び出し側は errors.Is(err, ErrNotFound) だけで判定できる。

func (e *NotFoundError) Is(target error) bool {
	return target == ErrNotFound
}

// ============================================================
// 4. エラーコード / Fault 分類（AWS SDK v2 / smithy 風）
// ============================================================

func (e *NotFoundError) ErrorCode() string { return "NotFoundError" }

type ErrorFault int

const (
	FaultUnknown ErrorFault = iota
	FaultClient
	FaultServer
)

func (e *NotFoundError) ErrorFault() ErrorFault { return FaultClient }

// ============================================================
// 5. 自己申告型エラー
// ============================================================
//
// エラー型自身が NotFound() bool を実装し、横断的な判定に参加する。

type NotFoundReporter interface {
	error
	NotFound() bool
}

func (e *NotFoundError) NotFound() bool { return true }

// ============================================================
// 6. 横断判定
// ============================================================
//
// k8s apimachinery errors.IsNotFound と同じく、複数の契約を一つの関数で判定。

func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}

	var nf *NotFoundError
	if errors.As(err, &nf) {
		return true
	}

	var reporter NotFoundReporter
	if errors.As(err, &reporter) && reporter.NotFound() {
		return true
	}

	return false
}

// ============================================================
// 7. MaskNotFound（ent 風）
// ============================================================

func MaskNotFound(err error) error {
	if IsNotFound(err) {
		return nil
	}
	return err
}

// ============================================================
// 8. 低レイヤーエラーコード → ドメインエラー
// ============================================================
//
// syscall.Errno.Is が ENOENT を fs.ErrNotExist にマッピングするのと同じく、
// lib/pq の SQLState を自前の NotFoundError に変換する。

func MapPQError(err error) error {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return err
	}

	switch pqErr.SQLState() {
	case "42P01": // undefined_table
		return fmt.Errorf("pq %s: %w", pqErr.SQLState(), NewNotFoundError("table", pqErr.Table))
	default:
		return err
	}
}

// ============================================================
// 9. HTTP ステータスへの変換
// ============================================================

func ToHTTPStatus(err error) int {
	switch {
	case IsNotFound(err),
		errors.Is(err, sql.ErrNoRows),
		errors.Is(err, fs.ErrNotExist),
		errors.Is(err, redis.Nil):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// ============================================================
// 10. エラー型を使わない表現
// ============================================================

// Option[T]（samber/mo.Option と同じ考え方）
type Option[T any] struct {
	present bool
	value   T
}

func Some[T any](v T) Option[T] { return Option[T]{present: true, value: v} }
func None[T any]() Option[T]    { return Option[T]{} }

func (o Option[T]) IsSome() bool      { return o.present }
func (o Option[T]) IsNone() bool      { return !o.present }
func (o Option[T]) Get() (T, bool) {
	if !o.present {
		var zero T
		return zero, false
	}
	return o.value, true
}

// 番兵値 → Option（samber/mo.TupleToOption / PointerToOption 風）
func TupleToOption[T any](v T, ok bool) Option[T] {
	if ok {
		return Some(v)
	}
	return None[T]()
}

func PointerToOption[T any](p *T) Option[T] {
	if p == nil {
		return None[T]()
	}
	return Some(*p)
}

// ============================================================
// 11. ドメインモデルとリポジトリ
// ============================================================

type User struct {
	ID       string
	Name     string
	Nickname string
}

type UserRepo struct{ db *sql.DB }

func NewUserRepo(db *sql.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) queryUser(ctx context.Context, id string) (*User, error) {
	row := r.db.QueryRowContext(ctx, "SELECT id, name, nickname FROM users WHERE id = $1", id)

	var u User
	if err := row.Scan(&u.ID, &u.Name, &u.Nickname); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// database/sql.ErrNoRows を ent/k8s 風の型付きエラーにラップする。
			return nil, fmt.Errorf("users repo: %w", NewNotFoundError("users", id))
		}
		return nil, err
	}
	return &u, nil
}

// GetByID は型付きエラーを返す（ent/gorm 風）。
func (r *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
	return r.queryUser(ctx, id)
}

// LookupByID は comma ok 形式（os.LookupEnv / sync.Map.Load と同じ）。
func (r *UserRepo) LookupByID(ctx context.Context, id string) (*User, bool, error) {
	u, err := r.queryUser(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return u, true, nil
}

// FindByID は Option[T] 形式（samber/mo.Option と同じ）。
func (r *UserRepo) FindByID(ctx context.Context, id string) (Option[*User], error) {
	u, err := r.queryUser(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			return None[*User](), nil
		}
		return None[*User](), err
	}
	return Some(u), nil
}

// Exists は redis.Exists / viper.IsSet と同じく、存在確認専用 API。
func (r *UserRepo) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", id,
	).Scan(&exists)
	return exists, err
}

// List は「無い場合は空スライス」を返す（mongo Find / sql Rows と同じ）。
func (r *UserRepo) List(ctx context.Context) ([]User, error) {
	// 存在しない UUID で必ず 0 件になるようにする。
	rows, err := r.db.QueryContext(ctx, "SELECT id, name, nickname FROM users WHERE id = $1", uuid.NewString())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Name, &u.Nickname); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

type Cache struct{ rdb *redis.Client }

func NewCache(rdb *redis.Client) *Cache { return &Cache{rdb: rdb} }

// Get は redis.Nil（センチネル）を自前の ErrNotFound に変換する。
func (c *Cache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", fmt.Errorf("cache repo: %w", ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	return val, nil
}

// Exists は redis.Exists と同じく 0/1 で存在を返す。
func (c *Cache) Exists(ctx context.Context, key string) (int64, error) {
	return c.rdb.Exists(ctx, key).Result()
}

// ============================================================
// 12. HTTP ハンドラ
// ============================================================

func handleGetUser(repo *UserRepo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			id = strings.TrimPrefix(r.URL.Path, "/users/")
		}

		u, err := repo.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), ToHTTPStatus(err))
			return
		}
		fmt.Fprintf(w, "user: %s\n", u.Name)
	}
}

func handleGetCache(cache *Cache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		_, err := cache.Get(r.Context(), key)
		if err != nil {
			http.Error(w, err.Error(), ToHTTPStatus(err))
			return
		}
		fmt.Fprintln(w, "cache hit")
	}
}

// ============================================================
// 13. インターフェース層での契約
// ============================================================
//
// ent/gorm のように「Store.Get は NotFound エラーを返す」という契約を、
// interface として定義し、実装側に押し付ける。

type UserStore interface {
	Get(ctx context.Context, id string) (*User, error)
	Exists(ctx context.Context, id string) (bool, error)
	Lookup(ctx context.Context, id string) (*User, bool, error)
}

// RedisUserStore はセンチネル契約（ErrNotFound を返す）。
type RedisUserStore struct{ rdb *redis.Client }

func NewRedisUserStore(rdb *redis.Client) *RedisUserStore { return &RedisUserStore{rdb: rdb} }

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
	return n > 0, err
}

func (s *RedisUserStore) Lookup(ctx context.Context, id string) (*User, bool, error) {
	u, err := s.Get(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return u, true, nil
}

// PostgresUserStore は型付きエラー契約（*NotFoundError を返す）。
type PostgresUserStore struct{ repo *UserRepo }

func NewPostgresUserStore(repo *UserRepo) *PostgresUserStore { return &PostgresUserStore{repo: repo} }

func (s *PostgresUserStore) Get(ctx context.Context, id string) (*User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *PostgresUserStore) Exists(ctx context.Context, id string) (bool, error) {
	return s.repo.Exists(ctx, id)
}

func (s *PostgresUserStore) Lookup(ctx context.Context, id string) (*User, bool, error) {
	return s.repo.LookupByID(ctx, id)
}

// MemoryArticleStore は自己申告契約（自前のエラー型に NotFound() bool）。

type ArticleStore interface {
	Get(ctx context.Context, id string) (*Article, error)
	Exists(ctx context.Context, id string) (bool, error)
	Lookup(ctx context.Context, id string) (*Article, bool, error)
}

type Article struct {
	ID    string
	Title string
}

type articleNotFoundError struct{ ID string }

func (e *articleNotFoundError) Error() string { return fmt.Sprintf("article %q not found", e.ID) }
func (e *articleNotFoundError) NotFound() bool { return true }

type MemoryArticleStore struct{ articles map[string]*Article }

func NewMemoryArticleStore() *MemoryArticleStore {
	return &MemoryArticleStore{articles: make(map[string]*Article)}
}

func (s *MemoryArticleStore) Get(ctx context.Context, id string) (*Article, error) {
	if a, ok := s.articles[id]; ok {
		return a, nil
	}
	return nil, &articleNotFoundError{ID: id}
}

func (s *MemoryArticleStore) Exists(ctx context.Context, id string) (bool, error) {
	_, ok := s.articles[id]
	return ok, nil
}

func (s *MemoryArticleStore) Lookup(ctx context.Context, id string) (*Article, bool, error) {
	if a, ok := s.articles[id]; ok {
		return a, true, nil
	}
	return nil, false, nil
}

// AssertUserStore は、UserStore を実装していればどの契約を使っていても OK であることを
// コンパイル時に保証する（database/sql driver tests で使われる conformance test 思想）。
func AssertUserStore(_ UserStore) {}

func handleGetFromStore(store UserStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		u, err := store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), ToHTTPStatus(err))
			return
		}
		fmt.Fprintf(w, "user: %s\n", u.Name)
	}
}

func handleGetArticle(store ArticleStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/articles/")
		a, err := store.Get(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), ToHTTPStatus(err))
			return
		}
		fmt.Fprintf(w, "article: %s\n", a.Title)
	}
}

// ============================================================
// 14. デモ
// ============================================================

type deps struct {
	db    *sql.DB
	rdb   *redis.Client
	repo  *UserRepo
	cache *Cache
}

func main() {
	ctx := context.Background()

	d := deps{
		db:  openPostgres(),
		rdb: openRedis(),
	}
	d.repo = NewUserRepo(d.db)
	d.cache = NewCache(d.rdb)

	demoErrorPatterns(ctx, d)
	demoNoErrorPatterns(ctx, d)
	demoInterfacePatterns(ctx, d)
}

func openPostgres() *sql.DB {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("PGHOST", "localhost"),
		getEnv("PGPORT", "5432"),
		getEnv("PGUSER", "postgres"),
		getEnv("PGPASSWORD", "postgres"),
		getEnv("PGDATABASE", "postgres"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		panic(err)
	}
	return db
}

func openRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr: getEnv("REDIS_ADDR", "localhost:6379"),
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		panic(err)
	}
	return rdb
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func demoErrorPatterns(ctx context.Context, d deps) {
	fmt.Println("== エラー型を使う表現 ==")

	id := uuid.NewString()

	// 1. 型付きエラー + errors.Is 拡張
	_, err := d.repo.GetByID(ctx, id)
	fmt.Printf("GetByID => IsNotFound=%v HTTP=%d err=%v\n", IsNotFound(err), ToHTTPStatus(err), err)

	// 2. redis.Nil センチネル
	_, err = d.cache.Get(ctx, "cache:user:"+id)
	fmt.Printf("cache.Get => IsNotFound=%v HTTP=%d err=%v\n", IsNotFound(err), ToHTTPStatus(err), err)

	// 3. ErrorCode / ErrorFault
	var nf *NotFoundError
	if errors.As(err, &nf) {
		fmt.Printf("ErrorCode=%s ErrorFault=%d\n", nf.ErrorCode(), nf.ErrorFault())
	}

	// 4. MaskNotFound
	masked := MaskNotFound(err)
	fmt.Printf("MaskNotFound => nil? %v\n", masked == nil)

	// 5. fs.ErrNotExist（標準ライブラリのセンチネル）
	fmt.Printf("fs.ErrNotExist => HTTP=%d\n", ToHTTPStatus(fs.ErrNotExist))

	// 6. lib/pq SQLState → ドメインエラー
	pqErr := MapPQError(&pq.Error{Code: "42P01"})
	fmt.Printf("MapPQError => IsNotFound=%v HTTP=%d err=%v\n", IsNotFound(pqErr), ToHTTPStatus(pqErr), pqErr)

	fmt.Println()
}

func demoNoErrorPatterns(ctx context.Context, d deps) {
	fmt.Println("== エラー型を使わない表現 ==")

	id := uuid.NewString()

	// comma ok（os.LookupEnv / sync.Map.Load と同じ）
	u, ok, err := d.repo.LookupByID(ctx, id)
	fmt.Printf("LookupByID => ok=%v user=%v err=%v\n", ok, u, err)

	// Option[T]（samber/mo.Option と同じ）
	opt, err := d.repo.FindByID(ctx, id)
	optVal, optOk := opt.Get()
	fmt.Printf("FindByID => IsNone=%v Get=<%v,%v> err=%v\n", opt.IsNone(), optVal, optOk, err)

	// 番兵値 → Option（PointerToOption 風）
	var ptr *User
	opt2 := PointerToOption(ptr)
	fmt.Printf("PointerToOption(nil) => IsNone=%v\n", opt2.IsNone())

	// Exists API（redis.Exists / viper.IsSet と同じ）
	exists, err := d.repo.Exists(ctx, id)
	fmt.Printf("Exists => %v\n", exists)

	// 空コレクション（mongo Find / sql Rows と同じ）
	users, err := d.repo.List(ctx)
	fmt.Printf("List => %d items err=%v\n", len(users), err)

	// 番兵値（strings.Index の -1 と同じ）
	idx := strings.Index("hello", "z")
	fmt.Printf("strings.Index => %d\n", idx)

	// redis.Exists の 0/1
	n, _ := d.cache.Exists(ctx, "nonexistent:key")
	fmt.Printf("redis Exists => %d\n", n)

	// Null 型（sql.NullString と同じ）
	var ns sql.NullString
	fmt.Printf("NullString => Valid=%v String=%q\n", ns.Valid, ns.String)

	// http.NotFound helper
	rec := httptest.NewRecorder()
	http.NotFound(rec, nil)
	fmt.Printf("http.NotFound => %d\n", rec.Code)

	fmt.Println()
}

func demoInterfacePatterns(ctx context.Context, d deps) {
	fmt.Println("== インターフェース層での契約 ==")

	id := uuid.NewString()

	// 型付きエラー契約
	pgStore := NewPostgresUserStore(d.repo)
	AssertUserStore(pgStore)
	rec := httptest.NewRecorder()
	handleGetFromStore(pgStore)(rec, httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	fmt.Printf("postgres (型付きエラー契約) => %d %s", rec.Code, rec.Body.String())

	// センチネル契約
	redisStore := NewRedisUserStore(d.rdb)
	AssertUserStore(redisStore)
	rec = httptest.NewRecorder()
	handleGetFromStore(redisStore)(rec, httptest.NewRequest(http.MethodGet, "/users/"+id, nil))
	fmt.Printf("redis    (センチネル契約) => %d %s", rec.Code, rec.Body.String())

	// 自己申告契約
	articleStore := NewMemoryArticleStore()
	rec = httptest.NewRecorder()
	handleGetArticle(articleStore)(rec, httptest.NewRequest(http.MethodGet, "/articles/"+id, nil))
	fmt.Printf("memory   (自己申告契約) => %d %s", rec.Code, rec.Body.String())
}
