package util

import "log"

func Ensure(err error) error {
	log.Fatalln(err)
	return err
}

func BindingModelException(err error) error{
	log.Fatalln(err)
	return  err
}
