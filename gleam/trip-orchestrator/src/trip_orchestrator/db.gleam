import gleam/erlang/process
import gleam/int
import gleam/option.{None, Some}
import gleam/result
import pog

@external(erlang, "gleam_trip_orchestrator_ffi", "get_env")
fn get_env(name: String) -> Result(String, Nil)

pub type DbConfig {
  DbConfig(
    host: String,
    port: Int,
    database: String,
    user: String,
    password: String,
    pool_size: Int,
  )
}

pub fn default_config() -> DbConfig {
  let host = get_env("CAIRN_DB_HOST") |> result.unwrap("localhost")
  let port =
    get_env("CAIRN_DB_PORT")
    |> result.try(int.parse)
    |> result.unwrap(5432)
  let database = get_env("CAIRN_DB_NAME") |> result.unwrap("cairn")
  let user = get_env("CAIRN_DB_USER") |> result.unwrap("cairn")
  let password = get_env("CAIRN_DB_PASSWORD") |> result.unwrap("")
  let pool_size =
    get_env("CAIRN_DB_POOL_SIZE")
    |> result.try(int.parse)
    |> result.unwrap(10)

  DbConfig(host:, port:, database:, user:, password:, pool_size:)
}

pub fn connect(config: DbConfig) -> Result(pog.Connection, String) {
  let password_opt = case config.password {
    "" -> None
    pw -> Some(pw)
  }

  let pool_name = process.new_name(prefix: "cairn_db")

  let pog_config =
    pog.default_config(pool_name:)
    |> pog.host(config.host)
    |> pog.port(config.port)
    |> pog.database(config.database)
    |> pog.user(config.user)
    |> pog.password(password_opt)
    |> pog.pool_size(config.pool_size)

  case pog.start(pog_config) {
    Ok(started) -> Ok(started.data)
    Error(_) -> Error("Failed to start database connection pool")
  }
}
