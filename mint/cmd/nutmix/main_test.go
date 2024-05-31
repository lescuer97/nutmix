package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMintSuccessBolt11(t *testing.T) {

	const posgrespassword = "password"
	const postgresuser = "user"
	ctx := context.Background()

	postgresContainer, err := postgres.RunContainer(ctx,
		testcontainers.WithImage("postgres"),
		postgres.WithDatabase("postgres"),
		postgres.WithUsername(postgresuser),
		postgres.WithPassword(posgrespassword),
	)
	if err != nil {
		t.Fatal(err)
	}

    connUri, err := postgresContainer.ConnectionString(ctx)
    
	if err != nil {
		t.Fatal(fmt.Errorf("failed to get connection string: %s", err))
	}


    fmt.Printf("connURI %s: ", connUri)
    os.Setenv("DATABASE_URL", connUri)

	pool, err := DatabaseSetup()

    // var DATABASE_URL = "postgres://postgres:admin@localhost:5432/postgres"

	// Clean up the container
	defer func() {
		if err := postgresContainer.Terminate(ctx); err != nil {
			log.Fatalf("failed to terminate container: %s", err)
		}

	}()

}
