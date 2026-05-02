package usecase

import "github.com/otakakot/playground-golang/internal/di/repository"

type UseCase struct {
	repo repository.Repositry
}

func NewUseCase(repo repository.Repositry) *UseCase {
	return &UseCase{repo: repo}
}

func (uc *UseCase) A() error {
	return uc.repo.A()
}
