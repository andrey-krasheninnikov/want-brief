use anyhow::{anyhow, Context, Result};
use sqlx::{Connection, Executor, PgConnection};
use std::fs;
use std::path::{Path, PathBuf};

const LOCK_KEY: i64 = 0x77616e7462726965;
const V48_PREPARATION_END: &str = "-- end prepare-workspaces\n";

struct Migration {
    version: i64,
    path: PathBuf,
    up: String,
}

pub async fn migrate(database_url: &str, directory: &Path) -> Result<()> {
    let migrations = load_migrations(directory)?;
    let mut connection = PgConnection::connect(database_url)
        .await
        .context("connect for migrations")?;
    sqlx::query("SELECT pg_advisory_lock($1)")
        .bind(LOCK_KEY)
        .execute(&mut connection)
        .await
        .context("lock migrations")?;
    let result = apply(&mut connection, &migrations).await;
    let unlocked = sqlx::query("SELECT pg_advisory_unlock($1)")
        .bind(LOCK_KEY)
        .execute(&mut connection)
        .await;
    result?;
    unlocked.context("unlock migrations")?;
    Ok(())
}

async fn current_version(connection: &mut PgConnection) -> Result<i64> {
    let version = sqlx::query_scalar(
        "SELECT coalesce(max(version_id), 0) FROM (\
         SELECT DISTINCT ON (version_id) version_id, is_applied \
         FROM goose_db_version ORDER BY version_id, id DESC\
         ) versions WHERE is_applied",
    )
    .fetch_one(&mut *connection)
    .await?;
    Ok(version)
}

/// Expand a single-owner v48 installation while its API is still serving requests.
pub async fn prepare_workspaces(database_url: &str, directory: &Path) -> Result<()> {
    let migrations = load_migrations(directory)?;
    let preparation = migrations
        .iter()
        .find(|migration| migration.version == 50)
        .and_then(|migration| migration.up.split_once(V48_PREPARATION_END))
        .map(|(preparation, _)| preparation)
        .context("migration 50 has no workspace preparation stage")?;
    let mut connection = PgConnection::connect(database_url)
        .await
        .context("connect for preparation")?;
    let mut tx = connection.begin().await?;
    sqlx::query("SELECT pg_advisory_xact_lock($1)")
        .bind(LOCK_KEY)
        .execute(&mut *tx)
        .await?;
    anyhow::ensure!(
        current_version(&mut tx).await? == 48,
        "prepare-workspaces requires schema 48"
    );
    sqlx::query("LOCK TABLE users IN SHARE MODE")
        .execute(&mut *tx)
        .await?;
    let users: i64 = sqlx::query_scalar("SELECT count(*) FROM users")
        .fetch_one(&mut *tx)
        .await?;
    anyhow::ensure!(
        users == 1,
        "prepare-workspaces requires exactly one installation owner"
    );
    sqlx::raw_sql(preparation)
        .execute(&mut *tx)
        .await
        .context("prepare workspace ownership")?;
    tx.commit().await?;
    Ok(())
}

async fn apply(connection: &mut PgConnection, migrations: &[Migration]) -> Result<()> {
    connection
        .execute(
            "CREATE TABLE IF NOT EXISTS goose_db_version (\
             id SERIAL PRIMARY KEY, version_id BIGINT NOT NULL, \
             is_applied BOOLEAN NOT NULL, tstamp TIMESTAMP NOT NULL DEFAULT now())",
        )
        .await
        .context("create migration journal")?;
    let journal_rows: i64 = sqlx::query_scalar("SELECT count(*) FROM goose_db_version")
        .fetch_one(&mut *connection)
        .await?;
    if journal_rows == 0 {
        sqlx::query("INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, true)")
            .execute(&mut *connection)
            .await?;
    }
    let current = current_version(connection).await?;
    if current > migrations.last().map_or(0, |migration| migration.version) {
        return Err(anyhow!(
            "database migration version is newer than this binary"
        ));
    }
    for migration in migrations
        .iter()
        .filter(|migration| migration.version > current)
    {
        let mut transaction = connection.begin().await?;
        sqlx::raw_sql(&migration.up)
            .execute(&mut *transaction)
            .await
            .with_context(|| format!("apply {}", migration.path.display()))?;
        sqlx::query("INSERT INTO goose_db_version (version_id, is_applied) VALUES ($1, true)")
            .bind(migration.version)
            .execute(&mut *transaction)
            .await?;
        transaction.commit().await?;
    }
    Ok(())
}

fn load_migrations(directory: &Path) -> Result<Vec<Migration>> {
    let mut migrations = Vec::new();
    for entry in fs::read_dir(directory).with_context(|| format!("read {}", directory.display()))? {
        let path = entry?.path();
        if path.extension().is_none_or(|extension| extension != "sql") {
            continue;
        }
        let name = path
            .file_name()
            .and_then(|value| value.to_str())
            .ok_or_else(|| anyhow!("invalid migration filename"))?;
        let version = name
            .split_once('_')
            .ok_or_else(|| anyhow!("invalid migration filename: {name}"))?
            .0
            .parse::<i64>()
            .with_context(|| format!("invalid migration version: {name}"))?;
        let contents = fs::read_to_string(&path)?;
        let up = contents
            .split_once("-- +goose Up\n")
            .and_then(|(_, rest)| rest.split_once("-- +goose Down\n"))
            .ok_or_else(|| anyhow!("missing migration section: {name}"))?
            .0
            .to_owned();
        migrations.push(Migration { version, path, up });
    }
    migrations.sort_by_key(|migration| migration.version);
    if migrations.is_empty() {
        return Err(anyhow!("no migrations in {}", directory.display()));
    }
    if migrations
        .windows(2)
        .any(|pair| pair[0].version == pair[1].version)
    {
        return Err(anyhow!("duplicate migration version"));
    }
    Ok(migrations)
}

#[cfg(test)]
mod tests {
    use super::load_migrations;
    use std::path::Path;

    #[test]
    fn reads_existing_migrations_in_version_order() {
        let migrations = load_migrations(Path::new("migrations")).unwrap();
        assert_eq!(migrations.first().unwrap().version, 1);
        assert_eq!(migrations.last().unwrap().version, 52);
        assert!(migrations
            .iter()
            .all(|migration| !migration.up.trim().is_empty()));
    }
}
