package database

import (
	"github.com/google/uuid"
)

var devUserUUID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func GetDevUserID() uuid.UUID {
	return devUserUUID
}
