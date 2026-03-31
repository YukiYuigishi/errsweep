package usecase

import (
	"fmt"

	"example.com/myapp/repository"
)

func GetUser(id int) (string, error) {
	name, err := repository.FindByID(id)
	if err != nil {
		return "", fmt.Errorf("GetUser: %w", err)
	}
	return name, nil
}

func RegisterUser(name string) error {
	return repository.Create(name)
}
