package main

import (
	"context"
	"fmt"
	"testing"
)

func Good(
	ctx context.Context,
	is1 bool,
	is2 bool,
) (err error) {
	err = Func1(ctx, is1)
	if err != nil {
		return fmt.Errorf("good: %w", err)
	}

	defer func() {
		err = Func2(ctx, is2)
	}()

	fmt.Printf("good")

	return
}

func Bad(
	ctx context.Context,
	is1 bool,
	is2 bool,
) error {
	err := Func1(ctx, is1)
	if err != nil {
		return fmt.Errorf("bad: %w", err)
	}

	defer func() {
		err = Func2(ctx, is2)
	}()

	fmt.Printf("bad")

	return nil
}

func Func1(
	ctx context.Context,
	is bool,
) error {
	if is {
		return fmt.Errorf("func1 error")
	}

	return nil
}

func Func2(
	ctx context.Context,
	is bool,
) error {
	if is {
		return fmt.Errorf("func2 error")
	}

	return nil
}

func TestGood(t *testing.T) {
	type args struct {
		ctx context.Context
		is1 bool
		is2 bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "",
			args: args{
				ctx: context.Background(),
				is1: true,
			},
			wantErr: true,
		},
		{
			name: "",
			args: args{
				ctx: context.Background(),
				is1: false,
				is2: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Good(tt.args.ctx, tt.args.is1, tt.args.is2); (err != nil) != tt.wantErr {
				t.Errorf("Good() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBad(t *testing.T) {
	type args struct {
		ctx context.Context
		is1 bool
		is2 bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "",
			args: args{
				ctx: context.Background(),
				is1: true,
			},
			wantErr: true,
		},
		{
			name: "",
			args: args{
				ctx: context.Background(),
				is1: false,
				is2: true,
			},
			wantErr: true, // テストに失敗する
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Bad(tt.args.ctx, tt.args.is1, tt.args.is2); (err != nil) != tt.wantErr {
				t.Errorf("Bad() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
