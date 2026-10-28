use anyhow::{Context, Result};
use sqlx::PgPool;
use std::env;
use std::path::Path;
use std::sync::Arc;
use want_brief::auth::Auth;
use want_brief::crypto::TokenBox;
use want_brief::http::{router, AppState};
use want_brief::migrate::migrate;

fn setting(name: &str, fallback: &str) -> String {
    env::var(name)
        .ok()
        .filter(|value| !value.is_empty())
        .unwrap_or_else(|| fallback.to_owned())
}

#[tokio::main]
async fn main() -> Result<()> {
    let migrate_only = match env::args().skip(1).collect::<Vec<_>>().as_slice() {
        [] => false,
        [flag] if flag == "-migrate-only" || flag == "--migrate-only" => true,
        _ => anyhow::bail!("usage: want-brief [-migrate-only]"),
    };
    let database_url = setting(
        "DATABASE_URL",
        "postgres://wantbrief:wantbrief@127.0.0.1:5432/wantbrief?sslmode=disable",
    );
    let migrations_dir = setting("MIGRATIONS_DIR", "migrations");
    migrate(&database_url, Path::new(&migrations_dir)).await?;
    if migrate_only {
        return Ok(());
    }
    let token_key = setting(
        "TOKEN_KEY",
        "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff",
    );
    let token_box = Arc::new(TokenBox::new(&token_key).context("validate token key")?);
    let pool = PgPool::connect(&database_url)
        .await
        .context("connect database")?;
    let auth = Auth::new(pool.clone());
    auth.ensure_user(&setting("BOOTSTRAP_PASSWORD", "wantbrief"))
        .await?;
    let address = setting("HTTP_ADDR", ":8080");
    let bind_address = if address.starts_with(':') {
        format!("0.0.0.0{address}")
    } else {
        address.clone()
    };
    let listener = tokio::net::TcpListener::bind(&bind_address)
        .await
        .context("bind HTTP address")?;
    let state = AppState {
        pool,
        auth,
        cors_origin: setting("CORS_ORIGIN", "http://127.0.0.1:5173"),
        token_box,
    };
    println!("want brief api on {address}");
    axum::serve(listener, router(state))
        .await
        .context("serve HTTP")?;
    Ok(())
}
