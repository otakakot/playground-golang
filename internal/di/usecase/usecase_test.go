package usecase_test

import (
	"fmt"
	"testing"

	"github.com/otakakot/playground-golang/internal/di/repository"
	"github.com/otakakot/playground-golang/internal/di/usecase"
)

type SuccessMock struct {
	repository.Gateway
}

func (m *SuccessMock) A() error {
	return nil
}

type FailureMock struct {
	repository.Gateway
}

func (m *FailureMock) A() error {
	return fmt.Errorf("error")
}

func TestUseCase_A(t *testing.T) {
	tests := []struct {
		name    string
		repo    func(t *testing.T) repository.Repositry
		wantErr bool
	}{
		{
			name: "success",
			repo: func(t *testing.T) repository.Repositry {
				t.Helper()

				return &SuccessMock{}
			},
			wantErr: false,
		},
		{
			name: "failure",
			repo: func(t *testing.T) repository.Repositry {
				t.Helper()

				return &FailureMock{}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := usecase.NewUseCase(tt.repo(t))

			if err := uc.A(); (err != nil) != tt.wantErr {
				t.Errorf("UseCase.A() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
