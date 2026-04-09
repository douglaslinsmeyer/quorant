env "local" {
  src = "file://migrations"
  url = "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable"
  dev = "docker://postgres/16/dev?search_path=public"
}

env "test" {
  src = "file://migrations"
  url = "postgres://quorant:quorant@localhost:5433/quorant_test?sslmode=disable"
  dev = "docker://postgres/16/dev?search_path=public"
}
