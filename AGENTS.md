# Recall

Recall is a daemon that agents built on top of large language models can use to save useful bits of information.

## Project overview

Users will start Recall by running a daemon named `recalld`.

All data tracked by Recall will be stored in a single SQLite database.

## Technology stack

- Go 1.26
- SQLite 3.51

## Entity hierarchy

There are two primary entities in Recall: Projects and Memories. There is a one-to-many relationship between a Project and Memories.

### `project` table structure

| Column       |  Type  | Nullable | Notes                  |
| ------------ | :----: | :------: | ---------------------- |
| `id`         | `int`  |    No    | Primary Key            |
| `created_at` | `int`  |    No    | Creation timestamp     |
| `updated_at` | `int`  |    No    | Last updated timestamp |
| `name`       | `text` |    No    | Project name           |
| `name`       | `text` |    No    | Project name           |
