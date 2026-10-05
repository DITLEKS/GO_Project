package main

import (
	"fmt"
	"os"
	"runtime"
)

func username() string {
	if value := os.Getenv("USER"); value != "" {
		return value
	}
	if value := os.Getenv("USERNAME"); value != "" {
		return value
	}
	return "не определено"
}

func main() {
	fmt.Printf("Имя пользователя: %s\n", username())
	fmt.Printf("Аргументы CLI: %v\n", os.Args[1:])
	fmt.Printf("Версия Go: %s\n", runtime.Version())
}
