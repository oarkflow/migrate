# go run main.go cli Package

## Features
- **Migration Operations:** Supports creating, altering, dropping tables, views, functions, triggers, etc.
- **SQL Generation:** Automatically generates SQL queries from migration definitions written in BCL.
- **Transaction Handling:** Wraps migration operations in transactions to ensure consistency.
- **Validation Checks:** Pre- and Post-migration validation commands ensure proper migration execution.
- **Seed Data Support:** Ability to generate and insert seed data for testing environments.
- **Seed File Creation:** CLI command to generate seed files for tables.
- **Auto-Seeding:** Optionally auto-seed tables after migrations using generated or custom seed files.
- **Custom History Drivers:** Supports file-based and database-based migration history storage.
- **Extensible Dialects:** Easily add or modify SQL dialects for different databases.
- **Verbose Logging:** CLI supports verbose output for debugging and transparency.

## CLI Commands
- **make:migration:** Creates a new migration file based on the operation type.
- **make:seed:** Creates a new seed file for a table.
- **migrate:** Applies all pending migration files, with optional auto-seeding.
- **migration:rollback:** Rolls back the last applied migration (or a specified number of steps).
- **migration:reset:** Resets migrations by rolling back all applied migrations.
- **migration:validate:** Validates the applied migration history against the migration files.
- **db:seed:** Runs seed files to populate tables, with optional truncation.
- **history:** Report on entire changes on an object across all migrations.

## Effectiveness
- **Reliability:** Ensures migrations are applied safely using checksum comparison and transactional operations.
- **Ease of Use:** Simple command-line interface with clear commands and descriptive error logging.
- **Flexibility:** Automatically adapts SQL generation based on target dialect (Postgres, MySQL, SQLite).
- **Extensibility:** Add new drivers, dialects, or history storage backends without changing core logic.

## Extendable HistoryDriver and DatabaseDriver
- **HistoryDriver:** Supports file-based storage by default and can be easily extended to use database storage.
- **DatabaseDriver:** Provides a unified interface for different SQL databases (MySQL, Postgres, SQLite), allowing custom drivers for other databases.
- **Extensibility:** Developers can add new drivers or modify existing ones without altering the core migration logic.

## Why Use This Package and Use Cases
Developers choose this package because it:
- Simplifies migration management with a clear, declarative syntax.
- Automatically generates optimized SQL queries for various databases.
- Handles migration history, rollback, and reinstallation seamlessly.
- Offers extendable drivers to cater to custom database or storage requirements.
- Eliminates the pain of manually adjusting SQL files during database migration—developers need not worry about differences between SQL dialects.
- Supports robust seeding and test data generation for development and CI/CD.

### Use Cases & Examples
1. **Rapid Development Setup:** Quickly create and apply migrations to set up new application schemas with a single CLI command.
2. **Continuous Integration:** Integrate the migration commands in CI/CD pipelines to ensure schema consistency in every environment.
3. **Database Upgrades:** Efficiently apply, rollback, or reset migrations during application upgrades with minimal downtime.
4. **Custom Extensions:** Easily extend or replace the default HistoryDriver/DatabaseDriver for specialized project needs.
5. **Automated Seeding:** Generate and run seed files for tables, supporting fake data and custom expressions.

Note: Developers do not need to worry about the pain of SQL file migration from one database to another; the package handles SQL generation differences automatically.

## Example Commands

### Create a New Migration File
Command:
```
$ go run main.go cli make:migration create_users_table
```
This creates a file named similar to:
```bcl
// Example migration file generated by make:migration
Migration "1665678901_create_users_table" {
  Version = "1.0.0"
  Description = "Create table users."
  Connection = "default"
  Up {
    CreateTable "users" {
      Column "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Column "username" {
        type = "string"
        size = 100
        unique = true
      }
      # ...existing column definitions...
    }
  }
  Down {
    DropTable "users" {
      Cascade = true
    }
  }
}
```

### Create a New Seed File
Command:
```
$ go run main.go cli make:seed seo_metadatas
```
This creates a file named similar to:
```bcl
Seed "extendedTest" {
    table = "seo_metadatas"
    Field "id" {
        value = "fake_uuid"
        unique = true
    }
    Field "is_active" {
        value = true
    }
    Field "age" {
        value = "fake_age"
        data_type = "int"
    }
    Field "allowed_to_vote" {
        value = "expr: age.value > 20 ? true : false"
		data_type = "boolean"
    }
    Field "is_citizen" {
        value = "expr: allowed_to_vote.value ? true : false"
		data_type = "boolean"
    }
    combine = ["name", "status"]
    condition = "if_exists"
    rows = 2
}

```

### Apply Migrations (with optional seeding)
Command:
```
$ go run main.go cli migrate --seed=true --rows=10
```
This runs all pending migrations and seeds tables with 10 rows each.

### Run Seeds
Command:
```
$ go run main.go cli db:seed --file=path/to/seed_file.bcl --truncate=true
```
Runs the specified seed file, truncating the table before seeding.

### Rollback Migrations
Command (rollback last migration):
```
$ go run main.go cli migration:rollback --step=1
```

### Reset Migrations
Command:
```
$ go run main.go cli migration:reset
```

### Validate Migration History
Command:
```
$ go run main.go cli migration:validate
```

### Generate Migration History Report
Command:
```
$ go run main.go cli history [--object=users] [--serve=true]
```

#### Final Structure
![alt text](/assets/structure.png)

#### History
![alt text](/assets/history.png)