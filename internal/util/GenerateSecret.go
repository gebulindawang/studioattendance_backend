package util

import "golang.org/x/crypto/bcrypt"

func GenerateSecret(password string) (string , error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password),bcrypt.DefaultCost)
	if err != nil{
		return "加密失败", err
	}
	return  string(hash),nil
}