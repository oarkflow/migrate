package migrate

import (
	"strings"
	"testing"
)

func TestParseMigrationsBCLMultipleRoots(t *testing.T) {
	src := []byte(`
Migration "001_create_users" {
  Version = "1.0.0"
  Description = "Create users."
  Connection = "default"
  Up {
    CreateTable "users" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "email" {
        type = "string"
        unique = true
      }
    }
  }
  Down {
    DropTable "users" {
      Cascade = true
    }
  }
}

Migration "002_add_profile" {
  Version = "1.0.0"
  Description = "Add profile."
  Connection = "default"
  Up {
    AlterTable "users" {
      AddField {
        name = "profile"
        type = "text"
        nullable = true
      }
      RenameField "email_rename" {
        from = "email"
        to = "primary_email"
      }
    }
  }
  Down {
    AlterTable "users" {
      DropField "profile" {}
    }
  }
}
`)

	migrations, err := ParseMigrationsBCL(src)
	if err != nil {
		t.Fatalf("ParseMigrationsBCL returned error: %v", err)
	}
	if len(migrations) != 2 {
		t.Fatalf("len(migrations) = %d, want 2", len(migrations))
	}
	if migrations[0].Name != "001_create_users" {
		t.Fatalf("first migration name = %q", migrations[0].Name)
	}
	if got := migrations[0].Up.CreateTable[0].AddFields[1].Name; got != "email" {
		t.Fatalf("second field name = %q, want email", got)
	}
	if !migrations[0].Up.CreateTable[0].AddFields[1].Unique {
		t.Fatalf("email field unique flag was not decoded")
	}
	if got := migrations[1].Up.AlterTable[0].AddFields[0].Name; got != "profile" {
		t.Fatalf("add field name = %q, want profile", got)
	}
	if got := migrations[1].Up.AlterTable[0].RenameFields[0].To; got != "primary_email" {
		t.Fatalf("rename target = %q, want primary_email", got)
	}
}

func TestParseMigrationsBCLDuplicateName(t *testing.T) {
	src := []byte(`
Migration "dup" {}
Migration "dup" {}
`)

	_, err := ParseMigrationsBCL(src)
	if err == nil {
		t.Fatal("expected duplicate migration error")
	}
	if !strings.Contains(err.Error(), `duplicate migration name "dup"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestParseSeedsBCLMultipleRoots(t *testing.T) {
	src := []byte(`
Seed "users_seed" {
  table = "users"
  Field "id" {
    value = "fake_uuid"
    unique = true
  }
  Field "active" {
    value = true
    data_type = "boolean"
  }
  rows = 2
}

Seed "products_seed" {
  table = "products"
  Field "sku" {
    value = "fake_string"
  }
  rows = 1
}
`)

	seeds, err := ParseSeedsBCL(src)
	if err != nil {
		t.Fatalf("ParseSeedsBCL returned error: %v", err)
	}
	if len(seeds) != 2 {
		t.Fatalf("len(seeds) = %d, want 2", len(seeds))
	}
	if seeds[0].Name != "users_seed" || seeds[0].Fields[0].Name != "id" {
		t.Fatalf("unexpected first seed: %#v", seeds[0])
	}
	if seeds[1].Table != "products" || seeds[1].Rows != 1 {
		t.Fatalf("unexpected second seed: %#v", seeds[1])
	}
}

func TestSeedDefinitionUsesLocalSeedFunctionRegistry(t *testing.T) {
	RegisterSeedFunction("fake_test_value", func(args ...any) (any, error) {
		return "registered-value", nil
	})

	seed := SeedDefinition{
		Name:  "test_seed",
		Table: "widgets",
		Rows:  1,
		Fields: []FieldDefinition{
			{Name: "name", Value: "fake_test_value"},
		},
	}
	queries, err := seed.ToSQL(DialectPostgres)
	if err != nil {
		t.Fatalf("ToSQL returned error: %v", err)
	}
	if len(queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1", len(queries))
	}
	if got := queries[0].Args["name"]; got != "registered-value" {
		t.Fatalf("seed arg = %v, want registered-value", got)
	}
}
