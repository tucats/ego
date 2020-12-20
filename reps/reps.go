package reps

import "github.com/google/uuid"

type User struct {
	Name        string    `json:"name"`
	ID          uuid.UUID `json:"id"`
	Password    string    `json:"password"`
	Permissions []string  `json:"permissions"`
}

type UserCollection struct {
	Count  int    `json:"count"`
	Items  []User `json:"items"`
	Status int    `json:"status"`
	Msg    string `json:"msg"`
}
