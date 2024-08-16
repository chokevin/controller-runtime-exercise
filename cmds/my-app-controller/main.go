package main

import (
	"context"

	"github.com/steeling/controller-runtime-exercise/pkg/controller"
)

func main() {
	ctx := context.Background()

	// Create a new controller
	c, err := controller.New(ctx)
	check(err)

	// Start the controller
	c.Start(ctx)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
