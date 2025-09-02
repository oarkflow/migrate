# Go Migration Tool

A powerful, flexible database migration tool for Go applications that supports multiple database engines and provides comprehensive migration management capabilities.

## 🚀 Features

### Core Migration Features
- **Multi-Database Support:** PostgreSQL, MySQL, and SQLite
- **BCL-Based Migrations:** Write migrations in a simple, declarative BCL format
- **Automatic SQL Generation:** Converts BCL definitions to database-specific SQL
- **Transaction Safety:** All migrations run within transactions for consistency
- **Rollback Support:** Safe rollback of migrations with validation
- **Migration History:** Track applied migrations with checksums for integrity
- **Dry Run Mode:** Preview migrations without applying them

### Advanced Features
- **Seed Data Management:** Generate and manage test data with fake data support
- **Schema Validation:** Comprehensive validation of migration files and database schemas
- **Configuration Management:** Flexible configuration with environment variable support
- **History Reporting:** Detailed HTML reports of schema changes over time
- **Extensible Architecture:** Plugin system for custom dialects and drivers
- **CLI Interface:** Rich command-line interface with verbose logging

### Data Types & Operations
- **Table Operations:** Create, alter, drop, rename tables
- **Field Operations:** Add, drop, rename fields with full type support
- **Index Management:** Create unique and regular indexes
- **Constraint Support:** Primary keys, foreign keys, check constraints
- **View Management:** Create, alter, drop views
- **Function/Procedure Support:** Database function and procedure management
- **Trigger Support:** Database trigger management

## 📦 Installation

```bash
go get github.com/oarkflow/migrate
```

## 🛠️ Quick Start

### 1. Initialize Configuration

```bash
go run main.go cli config:init
```

This creates a `migrate.json` configuration file with default settings.

### 2. Configure Database Connection

Edit `migrate.json`:

```json
{
  "database": {
    "driver": "postgres",
    "host": "localhost",
    "port": 5432,
    "username": "your_username",
    "password": "your_password",
    "database": "your_database"
  }
}
```

### 3. Create Your First Migration

```bash
go run main.go cli make:migration create_users_table
```

### 4. Edit the Migration File

The generated migration file will look like:

```bcl
Migration "1234567890_create_users_table" {
  Version = "1.0.0"
  Description = "Create users table"
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
        size = 255
        unique = true
      }
      Field "name" {
        type = "string"
        size = 100
      }
      Field "created_at" {
        type = "datetime"
        default = "now()"
      }
    }
  }
  Down {
    DropTable "users" {
      Cascade = true
    }
  }
}
```

### 5. Run Migrations

```bash
go run main.go cli migrate
```

## 📋 CLI Commands

### Migration Commands
- **`make:migration <name>`** - Create a new migration file
- **`migrate`** - Apply all pending migrations
- **`migration:rollback --step=<n>`** - Rollback n migrations
- **`migration:reset`** - Reset all migrations
- **`migration:validate`** - Validate migration files
- **`status`** - Show migration status

### Seed Commands
- **`make:seed <table>`** - Create a seed file for a table
- **`db:seed`** - Run all seed files
- **`db:seed --file=<path>`** - Run specific seed file
- **`db:seed --truncate=true`** - Truncate tables before seeding

### Configuration Commands
- **`config:init`** - Initialize configuration file
- **`config:validate`** - Validate configuration
- **`config:show`** - Display current configuration

### Reporting Commands
- **`history`** - Generate migration history report
- **`history --object=<name>`** - Report for specific object
- **`history --serve=true`** - Serve report via HTTP

## 🔧 Configuration

### Configuration File Structure

```json
{
  "database": {
    "driver": "postgres|mysql|sqlite",
    "host": "localhost",
    "port": 5432,
    "username": "user",
    "password": "pass",
    "database": "dbname",
    "ssl_mode": "disable",
    "timeout": 30
  },
  "migration": {
    "directory": "migrations",
    "table_name": "migrations",
    "lock_timeout": 300,
    "batch_size": 100,
    "auto_rollback": false,
    "dry_run": false,
    "skip_validation": false
  },
  "seed": {
    "directory": "migrations/seeds",
    "default_rows": 10,
    "truncate_first": false,
    "batch_size": 1000
  },
  "logging": {
    "level": "info",
    "format": "text",
    "output": "console",
    "verbose": false
  },
  "validation": {
    "enabled": true,
    "strict_mode": false,
    "max_identifier_length": 64,
    "require_description": true
  }
}
```

### Environment Variables

Override configuration with environment variables:

- `MIGRATE_DB_HOST` - Database host
- `MIGRATE_DB_PORT` - Database port
- `MIGRATE_DB_USERNAME` - Database username
- `MIGRATE_DB_PASSWORD` - Database password
- `MIGRATE_DB_DATABASE` - Database name
- `MIGRATE_DB_DRIVER` - Database driver
- `MIGRATE_MIGRATION_DIR` - Migration directory
- `MIGRATE_SEED_DIR` - Seed directory
- `MIGRATE_LOG_LEVEL` - Log level
- `MIGRATE_VERBOSE` - Enable verbose logging

## 📝 Migration Examples

### Creating Tables

```bcl
Migration "create_products_table" {
  Version = "1.0.0"
  Description = "Create products table with categories"
  Up {
    CreateTable "categories" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "name" {
        type = "string"
        size = 100
        unique = true
      }
    }

    CreateTable "products" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "name" {
        type = "string"
        size = 255
      }
      Field "price" {
        type = "decimal"
        size = 10
        scale = 2
      }
      Field "category_id" {
        type = "integer"
        foreign_key = {
          reference_table = "categories"
          reference_field = "id"
          on_delete = "CASCADE"
        }
      }
    }
  }
  Down {
    DropTable "products" { Cascade = true }
    DropTable "categories" { Cascade = true }
  }
}
```

### Altering Tables

```bcl
Migration "add_user_profile_fields" {
  Version = "1.0.0"
  Description = "Add profile fields to users table"
  Up {
    AlterTable "users" {
      AddField {
        name = "avatar_url"
        type = "string"
        size = 500
        nullable = true
      }
      AddField {
        name = "bio"
        type = "text"
        nullable = true
      }
      AddField {
        name = "is_verified"
        type = "boolean"
        default = false
      }
    }
  }
  Down {
    AlterTable "users" {
      DropField { name = "avatar_url" }
      DropField { name = "bio" }
      DropField { name = "is_verified" }
    }
  }
}
```

## 🌱 Seed Data

### Creating Seed Files

```bash
go run main.go cli make:seed users
```

### Seed File Example

```bcl
Seed "user_seed" {
  table = "users"

  Field "id" {
    value = "fake_uuid"
    unique = true
  }

  Field "email" {
    value = "fake_email"
    unique = true
  }

  Field "name" {
    value = "fake_name"
  }

  Field "age" {
    value = "fake_age"
    data_type = "int"
  }

  Field "is_active" {
    value = "expr: age.value > 18 ? true : false"
    data_type = "boolean"
  }

  rows = 50
  condition = "if_not_exists"
}
```

### Available Fake Data Functions

- `fake_uuid` - Generate UUID
- `fake_name` - Generate full name
- `fake_email` - Generate email address
- `fake_phone` - Generate phone number
- `fake_address` - Generate address
- `fake_company` - Generate company name
- `fake_date` - Generate date
- `fake_datetime` - Generate datetime
- `fake_age` - Generate age (1-100)
- `fake_bool` - Generate boolean
- `fake_string` - Generate random string
- `fake_int` - Generate integer
- `fake_float64` - Generate float

## 🏗️ Architecture

### Extensible Design

The migration tool is built with extensibility in mind:

- **Dialect System:** Easy to add support for new databases
- **Driver Interface:** Pluggable database drivers
- **History Drivers:** File-based or database-based history storage
- **Validation System:** Comprehensive validation with custom rules

### Database Support

| Database | Status | Features |
|----------|--------|----------|
| PostgreSQL | ✅ Full | All features supported |
| MySQL | ✅ Full | All features supported |
| SQLite | ✅ Partial | Limited ALTER support |

## 🧪 Testing

Run the test suite:

```bash
go test ./...
```

Run tests with coverage:

```bash
go test -cover ./...
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run the test suite
6. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.

## 🆘 Support

- **Documentation:** Check this README and inline code documentation
- **Issues:** Report bugs and request features via GitHub Issues
- **Examples:** See the `examples/` directory for usage examples

## 🔄 Migration Best Practices

1. **Always write reversible migrations** - Include proper Down operations
2. **Test migrations thoroughly** - Use dry-run mode first
3. **Keep migrations atomic** - One logical change per migration
4. **Use descriptive names** - Make migration purposes clear
5. **Validate before applying** - Use the validation commands
6. **Backup before major changes** - Especially in production
7. **Review generated SQL** - Use verbose mode to see actual queries

## 📊 Monitoring & Reporting

The tool provides comprehensive reporting capabilities:

- **Migration Status:** Track applied vs pending migrations
- **History Reports:** Visual HTML reports of schema evolution
- **Validation Reports:** Detailed validation results
- **Performance Metrics:** Track migration execution times

Generate a history report:

```bash
go run main.go cli history --object=users --serve=true
```

This will start a web server at `http://localhost:8080/history` with an interactive report.

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
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }
      Field "username" {
        type = "string"
        size = 100
        unique = true
      }
      # ...existing field definitions...
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
