package main

import (
	"fmt"

	"github.com/xhanio/errors"
)

// ERR_CONN is a base error for connection failures
var ERR_CONN = fmt.Errorf("failed to dail tcp")

// testconnection simulates a connection test that fails
func testconnection() error {
	return ERR_CONN
}

// connectdb attempts to connect to a database and wraps any errors with context
func connectdb() error {
	err := testconnection()
	if err != nil {
		// Wrapf adds formatted context to an existing error
		return errors.Wrapf(err, "failed to connect to db")
	}
	return nil
}

// querydb executes a database query and categorizes errors using error categories
func querydb() error {
	err := connectdb()
	if err != nil {
		// Wrap the error with a predefined category (DBFailed)
		return errors.DBFailed.Wrap(err)
	}
	// query on db
	return nil
}

// run demonstrates using a category as a standard error
func run() error {
	return errors.NotImplemented
}

// preflight demonstrates creating a new formatted error
func preflight() error {
	return errors.Newf("failed to initialize the project")
}

func main() {
	// Example 1: Simple error creation with Newf
	err := preflight()
	if err != nil {
		fmt.Printf("[ format with %%v ]\n%v\n", err)
	}

	// Example 2: Error wrapping and chain inspection
	err = querydb()
	if err != nil {
		// Different format verbs show different error representations
		fmt.Printf("[ format with %%m ]\n%m\n", err) // message only
		fmt.Printf("[ format with %%s ]\n%s\n", err) // string representation of error chain
		fmt.Printf("[ format with %%v ]\n%v\n", err) // verbose with full chain

		// Has checks if a specific error exists anywhere in the error chain
		if errors.Has(err, ERR_CONN) {
			fmt.Println("err chain contains ERR_CONN")
		}
		// Is checks if the error belongs to a specific category
		if errors.Is(err, errors.DBFailed) {
			fmt.Println("err is in DBFailed category")
		}
	}

	// Example 3: Using error categories directly
	err = run()
	if err != nil {
		fmt.Printf("[ format with %%v ]\n%v\n", err)
	}

}
