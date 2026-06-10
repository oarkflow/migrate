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

### Embedding Migrations into a Single Binary ✅

You can embed `migrations/`, `migrations/seeds/` and `templates/` into your Go binary using `//go:embed` and run migrations directly from the binary without shipping files:

```go
//go:embed migrations/** migrations/seeds/** templates/**
var assets embed.FS

mgr := migrate.NewManager(migrate.WithEmbeddedFiles(assets))
mgr.Run()
```

When `WithEmbeddedFiles` is used the tool will list and read migrations from the embedded filesystem.

> **Note:** Embedded assets are read-only at runtime. Creating new migration or seed files will write to the local filesystem and will not update the embedded assets inside the compiled binary.

## 📋 CLI Commands

### Migration Commands
- **`make:migration <name>`** - Create a new migration file
- **`make:migration <name> --raw=true`** - Create a raw SQL migration file
- **`migrate`** - Apply all pending BCL migrations
- **`migrate --include-raw=true`** - Apply pending BCL and raw SQL migrations
- **`migration:rollback --step=<n>`** - Rollback n migrations
- **`migration:rollback --step=<n> --force=true`** - Rollback and continue past statement errors
- **`migration:reset`** - Reset all migrations by running down operations
- **`migration:reset --force=true`** - Reset and continue past rollback statement errors
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

---

## 🧭 Migration Syntax Reference ✅

This section documents the full migration and seed structure, all fields, supported values and examples to help you write correct and validated migrations.

### Migration block

Top-level migration properties (inside `Migration "name" { ... }`):

- `Version` (string) — semantic or arbitrary version identifier. Required.
- `Description` (string) — short explanation of the migration. Required.
- `Connection` (string) — optional named connection to use (if you manage multiple connections).
- `Driver` (string) — optional driver override (e.g., `postgres`, `mysql`, `sqlite`).
- `Disable` (bool) — set to `true` to skip applying this migration.
- `Up` / `Down` (blocks) — an `Operation` block describing changes to apply and rollback respectively.
- `Transaction` (array) — optional transaction metadata (e.g., `IsolationLevel`).
- `Validate` (array) — optional pre/post checks (`PreUpChecks`, `PostUpChecks`).

Example header:

```bcl
Migration "20250101_add_users" {
  Version = "1.0.0"
  Description = "Create the users table and seed initial data"
  Connection = "default"
  Driver = "postgres"
  Up { /* operations */ }
  Down { /* rollback */ }
}
```

### Supported Operation types

Inside `Up` / `Down` you can use the following operations (short description):

- `CreateTable` — create a new table.
- `AlterTable` — add/drop/rename fields on an existing table.
- `DeleteData` — delete rows via a WHERE clause.
- `DropEnumType` — remove an enum type (Postgres).
- `DropRowPolicy` — remove row-level policy (Postgres).
- `DropMaterializedView` — drop a materialized view (Postgres).
- `DropTable` — drop a table (optionally cascade).
- `DropSchema` — drop a schema.
- `RenameTable` — rename a table.
- `CreateView`, `DropView`, `RenameView` — view management.
- `CreateFunction`, `DropFunction`, `RenameFunction` — function management.
- `CreateProcedure`, `DropProcedure`, `RenameProcedure` — stored procs.
- `CreateTrigger`, `DropTrigger`, `RenameTrigger` — triggers.

> Tip: Not all operations are supported or meaningful on every database dialect. The tool maps generic types and operations to dialect-specific SQL.

---

### Raw SQL migrations

Use raw SQL migrations when you need hand-written SQL, vendor-specific features, or statements not represented by BCL operations.

Create one with:

```bash
go run main.go cli make:migration add_user_indexes --raw=true
```

Raw migration files must use section comments:

```sql
-- migration-up
CREATE INDEX idx_users_email ON users (email);

-- migration-down
DROP INDEX IF EXISTS idx_users_email;
```

Rules:

- `-- migration-up` or `-- migrate-up` starts the apply section.
- `-- migration-down` or `-- migrate-down` starts the rollback section.
- `migrate` skips raw SQL migrations by default.
- `migrate --include-raw=true` executes the up section and stores history/checksum.
- `migration:rollback` and `migration:reset` execute only the down section.
- `migration:validate` requires a non-empty up section.
- Missing down SQL or rollback statement failures stop rollback/reset unless `--force=true` is used.
- Raw SQL files and BCL files can be mixed in the same migration directory; execution order is deterministic by filename, and history order controls rollback/reset.

---

### CreateTable

`CreateTable "name" { Field ... PrimaryKey = [ ... ] }`

- `Name` (string) — table name (required).
- `Field` entries are `AddField` objects (see next section).
- `PrimaryKey` (array of strings) — optional explicit primary-key columns. If omitted, any field with `primary_key = true` becomes part of primary key.

Example using both `PrimaryKey` and field-level `primary_key`:

```bcl
CreateTable "accounts" {
  Field "id" {
    type = "integer"
    primary_key = true
    auto_increment = true
  }

  Field "tenant_id" {
    type = "integer"
    primary_key = true
  }

  PrimaryKey = ["id", "tenant_id"]
}
```

---

### AddField / Field attributes

An `AddField` (written as `Field "colname" { ... }` or `AddField { ... }` inside `AlterTable`) supports the following attributes:

- `name` / field label — the column name (required).
- `type` (string) — generic type name, e.g. `string`, `integer`, `decimal`, `boolean`, `text`, `date`, `datetime`, `json`, `blob`, etc. See the `utils.ConvertType` mappings in code for dialect-specific translation.
- `size` (int, optional) — used for `varchar`, `string`, `decimal` length (e.g., `size = 255`).
- `scale` (int, optional) — used with `decimal`/`numeric` as precision scale.
- `nullable` (bool) — whether the column allows NULL. Default: `false` (not-null) unless set to `true`.
- `default` (any, optional) — default value. Common values: `now()` (mapped to `CURRENT_TIMESTAMP`), string values will be quoted automatically, `null` maps to `NULL`.
- `check` (string, optional) — CHECK constraint expression, e.g. `"price > 0"`.
- `auto_increment` (bool) — marks integer primary-like column as auto-increment (translated per dialect).
- `primary_key` (bool) — include the column in the primary key when `PrimaryKey` is not set.
- `unique` (bool) — create a unique index on the field.
- `index` (bool) — create a regular index on the field.
- `foreign_key` (object, optional) — nested foreign-key specification.

Example usage — types, defaults and constraints:

```bcl
CreateTable "products" {
  Field "sku" {
    type = "string"
    size = 64
    unique = true
  }

  Field "price" {
    type = "decimal"
    size = 10
    scale = 2
    check = "price >= 0"
    default = 0.00
  }

  Field "created_at" {
    type = "datetime"
    default = "now()"
  }
}
```

Notes on `default` handling:

- Strings are quoted automatically when `type = "string"`.
- `default = "now()"` becomes `CURRENT_TIMESTAMP` for supported dialects.
- `default = null` or `default = NULL` becomes `NULL`.

---

### Foreign keys

`foreign_key` is a nested object inside a `Field`/`AddField`:

- `reference_table` (string) — referenced table name (required).
- `reference_field` (string) — referenced column name (required).
- `on_delete` (string, optional) — action on delete (e.g., `CASCADE`, `SET NULL`, `RESTRICT`).
- `on_update` (string, optional) — action on update.

Example:

```bcl
Field "category_id" {
  type = "integer"
  foreign_key = {
    reference_table = "categories"
    reference_field = "id"
    on_delete = "CASCADE"
  }
}
```

---

### AlterTable specifics

`AlterTable "table" { AddField { ... } DropField { name = "..." } RenameField { from = "old" to = "new" } }`

- `AddField` uses the same attributes as `Field` in `CreateTable`.
- `DropField { name = "col" }` — drops the column.
- `RenameField { from = "old", to = "new" }` — renames a column. For Postgres and MySQL it generates `ALTER TABLE ... RENAME COLUMN ... TO ...`.

Example:

```bcl
AlterTable "users" {
  AddField {
    name = "is_verified"
    type = "boolean"
    default = false
  }

  RenameField {
    from = "username"
    to = "user_name"
  }

  DropField { name = "legacy_flag" }
}
```

> Note: SQLite has limited ALTER support — the tool will recreate tables when necessary (see implementation notes in the code).

---

### DropTable / Cascade

`DropTable "name" { Cascade = true }` — set `Cascade = true` to drop dependent objects (behavior depends on dialect).

---

### Transactions and Validation

- Use `Transaction` entries to control transaction behavior (e.g., isolation level). On Postgres the tool emits `BEGIN TRANSACTION ISOLATION LEVEL <level>` when configured.
- `Validate` entries allow you to specify `PreUpChecks` and `PostUpChecks` that the manager will evaluate before and after runs.

---

## 🌱 Seed Syntax Reference ✅

Seed files follow a `Seed "name" { ... }` structure.
With BCL v0.0.25 and later, a single `.bcl` file may contain multiple root
`Migration` blocks or multiple root `Seed` blocks; migrate applies them in file
order after sorting migration files by name.

Seed fields:

- `name` — seed name identifier.
- `table` — target table name (required).
- `Field` — array of `FieldDefinition` objects (see below).
- `rows` — number of rows to generate (default controlled by configuration if omitted).
- `combine` — optional list of column names to combine into unique values (used by some fake generators).
- `condition` — optional condition, e.g., `if_not_exists` or `if_exists`.

FieldDefinition attributes:

- `name` (string) — column name (required).
- `value` (any) — literal value, faker token (e.g., `fake_email`) or expression using `expr:` prefix (e.g., `expr: age.value > 18 ? true : false`).
- `unique` (bool) — attempt to ensure unique generated values for this field (generator will retry up to 100 times when necessary).
- `random` (bool) — treat `value` as a random generator placeholder; `random` interacts with internal fake/value helpers.
- `size` (int) — requested string size for fake generators.
- `data_type` (string) — cast/convert to a typed value (e.g., `int`, `boolean`).

Expressions and fake functions

- `value = "fake_email"` — uses built-in fakers (see list below).
- `value = "expr: <expression>"` — evaluated using the `expr` package; expressions can refer to other field values by `<field>.value`.

Example:

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

Available fake functions include: `fake_uuid`, `fake_name`, `fake_email`, `fake_phone`, `fake_address`, `fake_company`, `fake_date`, `fake_datetime`, `fake_age`, `fake_bool`, `fake_string`, `fake_int`, `fake_float64`.

---

### Quick reference: common generic type mapping examples

- `type = "string"`, `size = 100` → `VARCHAR(100)` on most dialects (default `VARCHAR(255)` if size omitted).
- `type = "decimal"`, `size = 10`, `scale = 2` → `NUMERIC(10,2)` or dialect-equivalent.
- `type = "integer"`, `auto_increment = true` → `SERIAL`/`BIGSERIAL` on Postgres or `AUTO_INCREMENT` on MySQL.

---

### Cheat-sheet: quick attribute reference 📋

| Attribute | Type | Description | Example |
|---|---:|---|---|
| `name` | string | Table/column/seed identifier (required where noted) | `users` |
| `type` | string | Generic data type; mapped per dialect (see `utils.ConvertType`) | `string`, `integer`, `decimal` |
| `size` | int | Length/precision for strings or decimals | `size = 255` |
| `scale` | int | Decimal scale/precision | `scale = 2` |
| `nullable` | bool | Allow NULL (default: NOT NULL unless set true) | `nullable = true` |
| `default` | any | Default value; `now()` → `CURRENT_TIMESTAMP`; strings auto-quoted for `type = "string"` | `default = "now()"` or `default = "guest"` |
| `check` | string | SQL CHECK constraint expression | `check = "price > 0"` |
| `auto_increment` | bool | Use DB auto-increment (map to SERIAL/AUTO_INCREMENT) | `auto_increment = true` |
| `primary_key` / `PrimaryKey` | bool / array | Column-level PK or explicit `PrimaryKey = ["id"]` array | `primary_key = true` or `PrimaryKey = ["id"]` |
| `unique` | bool | Create unique index on field | `unique = true` |
| `index` | bool | Create non-unique index on field | `index = true` |
| `foreign_key` | object | FK spec `{ reference_table, reference_field, on_delete, on_update }` | see example below |
| `rows` (seed) | int | Number of rows to generate | `rows = 10` |
| `value` (seed) | any | Literal, `fake_*` token, or `expr: <expression>` | `value = "fake_email"` or `value = "expr: age.value > 18 ? true : false"` |
| `data_type` (seed) | string | Cast/convert seed cell to a type (`int`, `boolean`) | `data_type = "int"` |
| `unique` (seed) | bool | Ensure generated values are unique (retries up to 100 attempts) | `unique = true` |
| `random` (seed) | bool | Use random generation (special handling in seed generator) | `random = true` |

---

### Comprehensive example — demonstrates most features 🔧

```bcl
Migration "20251201_full_example" {
  Version = "1.0.0"
  Description = "Full example covering CreateTable, AlterTable, FK, constraints, transactions and validation"
  Connection = "default"
  Driver = "postgres"

  Transaction {
    Name = "init"
    IsolationLevel = "SERIALIZABLE"
  }

  Validate {
    Name = "basic_checks"
    PostUpChecks = ["SELECT COUNT(*) FROM users" ]
  }

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

      Field "email" {
        type = "string"
        size = 255
        unique = true
        nullable = false
      }

      Field "age" {
        type = "integer"
        default = 18
      }

      Field "created_at" {
        type = "datetime"
        default = "now()"
      }

      Field "status" {
        type = "string"
        default = "active"
        check = "status IN ('active','inactive','banned')"
      }

      PrimaryKey = ["id"]
    }

    CreateTable "profiles" {
      Field "id" {
        type = "integer"
        primary_key = true
        auto_increment = true
      }

      Field "user_id" {
        type = "integer"
        foreign_key = {
          reference_table = "users"
          reference_field = "id"
          on_delete = "CASCADE"
          on_update = "CASCADE"
        }
      }

      Field "bio" {
        type = "text"
        nullable = true
      }

      Field "avatar_url" {
        type = "string"
        size = 500
        nullable = true
      }
    }

    AlterTable "users" {
      AddField {
        name = "phone"
        type = "string"
        size = 20
        unique = true
        nullable = true
      }

      RenameField {
        from = "username"
        to = "user_name"
      }
    }
  }

  Down {
    DropTable "profiles" { Cascade = true }
    AlterTable "users" {
      DropField { name = "phone" }
      RenameField { from = "user_name" to = "username" }
    }
    DropTable "users" { Cascade = true }
  }
}
```

### Seed example that demonstrates `fake_` tokens, `expr:` and unique handling 🌱

```bcl
Seed "full_user_seed" {
  table = "users"

  Field "id" {
    value = "fake_uuid"
    unique = true
  }

  Field "email" {
    value = "fake_email"
    unique = true
  }

  Field "user_name" {
    value = "fake_name"
    size = 50
  }

  Field "age" {
    value = "fake_age"
    data_type = "int"
  }

  Field "is_adult" {
    value = "expr: age.value >= 18 ? true : false"
    data_type = "boolean"
  }

  Field "ref_code" {
    value = "random_ref_${ref(user_name)}"
    random = true
    unique = true
  }

  rows = 5
  condition = "if_not_exists"
}
```

Notes:
- Seed `expr:` values can reference other fields using `<field>.value` (evaluation resolves dependencies automatically; expressions that refer to missing fields will error).
- Seed `unique` attempts up to 100 retries to generate a unique value; if it cannot, an error is returned.
- SQLite: when your `AlterTable` requires dropping or renaming columns, the tool recreates the table behind the scenes to preserve compatibility.

---

If you'd like, I can also add a compact checklist table that maps each attribute to the exact struct/JSON name used by the library (helpful when authoring BCL files programmatically). Below is that mapping for quick reference.

---

### Migration structure — full hierarchical mapping 🔍

This mapping shows top-level migration attributes, the operation blocks you can use, and for each block the available fields and their corresponding Go struct / JSON tags. Use this when generating BCL programmatically or converting between JSON/BCL.

---

#### Migration (top-level)

- Migration object (Go: `Migration`, JSON tags shown)
  - `name` → `Migration.Name` (required)
  - `Version` → `Migration.Version` (required)
  - `Description` → `Migration.Description` (required)
  - `Connection` → `Migration.Connection` (optional)
  - `Driver` → `Migration.Driver` (optional)
  - `Disable` → `Migration.Disable` (optional)
  - `Transaction` → `Migration.Transaction` ([]Transaction)
    - Transaction fields: `Name`, `IsolationLevel` (JSON: `IsolationLevel`), `Mode`
  - `Validate` → `Migration.Validate` ([]Validation)
    - Validation fields: `Name`, `PreUpChecks` (`[]string`), `PostUpChecks` (`[]string`)
  - `Up` → `Migration.Up` (Operation)
  - `Down` → `Migration.Down` (Operation)

---

#### Operation (Up / Down blocks)

Operation (Go: `Operation`, JSON: `Up`/`Down`) contains arrays of operation blocks. Each operation maps to a specific struct:

- `CreateTable` → `Operation.CreateTable` (`[]CreateTable`)
- `AlterTable` → `Operation.AlterTable` (`[]AlterTable`)
- `DeleteData` → `Operation.DeleteData` (`[]DeleteData`)
- `DropEnumType` → `Operation.DropEnumType` (`[]DropEnumType`)
- `DropRowPolicy` → `Operation.DropRowPolicy` (`[]DropRowPolicy`)
- `DropMaterializedView` → `Operation.DropMaterializedView` (`[]DropMaterializedView`)
- `DropTable` → `Operation.DropTable` (`[]DropTable`)
- `DropSchema` → `Operation.DropSchema` (`[]DropSchema`)
- `RenameTable` → `Operation.RenameTable` (`[]RenameTable`)
- `CreateView` / `DropView` / `RenameView` → `CreateView` / `DropView` / `RenameView`
- `CreateFunction` / `DropFunction` / `RenameFunction`
- `CreateProcedure` / `DropProcedure` / `RenameProcedure`
- `CreateTrigger` / `DropTrigger` / `RenameTrigger`

---

#### CreateTable block (`CreateTable` / `CreateTable` struct)

- JSON: `CreateTable "<name>" { Field ... PrimaryKey = [...] }` maps to Go `CreateTable`:
  - `Name` → `CreateTable.Name` (the block's label)
  - `Field` → `CreateTable.AddFields` (array of `AddField`)
  - `PrimaryKey` → `CreateTable.PrimaryKey` (`[]string`)

AddField (field-level properties) — Go struct `AddField` / JSON keys shown:
- `name` → `AddField.Name`
- `type` → `AddField.Type`
- `size` → `AddField.Size`
- `scale` → `AddField.Scale`
- `nullable` → `AddField.Nullable`
- `default` → `AddField.Default` (any)
- `check` → `AddField.Check`
- `auto_increment` → `AddField.AutoIncrement`
- `primary_key` → `AddField.PrimaryKey`
- `unique` → `AddField.Unique`
- `index` → `AddField.Index`
- `foreign_key` → `AddField.ForeignKey` (object)
  - `reference_table` → `ForeignKey.ReferenceTable`
  - `reference_field` → `ForeignKey.ReferenceField`
  - `on_delete` → `ForeignKey.OnDelete`
  - `on_update` → `ForeignKey.OnUpdate`

---

#### AlterTable block (`AlterTable` / `AlterTable` struct)

- JSON: `AlterTable "table" { AddField { ... } DropField { name = "..." } RenameField { from = "..." to = "..." } }`
- Go struct `AlterTable` fields:
  - `Name` → `AlterTable.Name`
  - `AddFields` → `AlterTable.AddFields` (`[]AddField`) — same `AddField` attributes as above
  - `DropFields` → `AlterTable.DropFields` (`[]DropField`)
    - `DropField.Name` (JSON `name`)
  - `RenameFields` → `AlterTable.RenameFields` (`[]RenameField`)
    - `RenameField.Name` (optional), `RenameField.From`, `RenameField.To`, `RenameField.Type`

Notes: SQLite special-case — renames/drops may trigger table recreation (see code)

---

#### DropTable / DropSchema / DropMaterializedView / DropFunction / DropProcedure / DropTrigger

Common fields:
- `name` → `{}.Name` (e.g., `DropTable.Name`)
- `cascade` → `DropTable.Cascade` (bool)
- `if_exists` → `*.IfExists` where supported

Example: `DropTable "users" { Cascade = true }` maps to `DropTable{Name: "users", Cascade: true}`

---

#### RenameTable / RenameView / RenameFunction / RenameProcedure / RenameTrigger

- Rename blocks carry `OldName` / `NewName` fields in Go (`OldName`, `NewName`) mapped from JSON `old_name` / `new_name`.

---

#### CreateView / CreateFunction / CreateProcedure / CreateTrigger

- Common fields:
  - `Name` (`name`) — object name
  - `Definition` (`definition`) — raw SQL/DDL body
  - `OrReplace` (`or_replace`) — optional boolean

---

#### DeleteData

- JSON: `DeleteData { Name = "table", Where = "id > 100" }`
- Maps to Go `DeleteData{Name, Where}`

---

#### Seed files (`Seed` / `SeedDefinition`)

Seed top-level:
- `name` → `SeedDefinition.Name`
- `table` → `SeedDefinition.Table`
- `Field` → `SeedDefinition.Fields` (`[]FieldDefinition`)
- `rows` → `SeedDefinition.Rows`
- `condition` → `SeedDefinition.Condition`
- `combine` → `SeedDefinition.Combine` (`[]string`)

FieldDefinition fields:
- `name` → `FieldDefinition.Name`
- `value` → `FieldDefinition.Value` (any) — supports `fake_*` tokens and `expr:` expressions
- `unique` → `FieldDefinition.Unique`
- `random` → `FieldDefinition.Random`
- `size` → `FieldDefinition.Size`
- `data_type` → `FieldDefinition.DataType`

---

If you'd like, I can also add a JSON/CSV export of this exact mapping under `examples/` so tools can consume it directly.

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

### Create a Raw SQL Migration File
Command:
```
$ go run main.go cli make:migration create_users_table --raw=true
```
This creates a `.sql` migration with explicit up/down sections:
```sql
-- migration-up
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL
);

-- migration-down
DROP TABLE IF EXISTS users;
```

Raw SQL migrations are applied only when you pass `--include-raw=true` to `migrate`. The `-- migration-up` section is required; `migration:validate` fails if it is missing or empty. The `-- migration-down` section is required for normal rollback/reset. If you intentionally want to remove history without a down section, use `--force=true`.

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
To additionally execute raw `.sql` migrations and raw `.sql` seed files located in your seed directory, append `--include-raw=true` to the command.

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
Rollback supports both `.bcl` and raw `.sql` migrations. For raw SQL, the tool executes the `-- migration-down` section. Rollback fails on missing down SQL or statement errors unless `--force=true` is provided.

### Reset Migrations
Command:
```
$ go run main.go cli migration:reset
```
Reset rolls back all applied migrations in reverse history order. It supports mixed `.bcl` and raw `.sql` histories. Use `--force=true` only when you explicitly want reset to continue after rollback errors.

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
