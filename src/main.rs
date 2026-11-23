use anyhow::{Context, Result};
use sqlx::PgPool;
use std::env;
use std::net::SocketAddr;
use std::path::Path;
use std::sync::Arc;
use uuid::Uuid;
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
    let args = env::args().skip(1).collect::<Vec<_>>();
    let database_url = setting(
        "DATABASE_URL",
        "postgres://wantbrief:wantbrief@127.0.0.1:5432/wantbrief?sslmode=disable",
    );
    let migrate_only = match args.as_slice() {
        [] => false,
        [flag] if flag == "-migrate-only" || flag == "--migrate-only" => true,
        [command, raw_id, plan]
            if command == "set-plan" && matches!(plan.as_str(), "free" | "paid") =>
        {
            let id = Uuid::parse_str(raw_id).context("invalid workspace ID")?;
            let pool = PgPool::connect(&database_url)
                .await
                .context("connect database")?;
            let updated = sqlx::query(
                "UPDATE workspaces SET plan=$2 WHERE id=$1 AND (NOT legacy_paid OR $2='paid')",
            )
            .bind(id)
            .bind(plan)
            .execute(&pool)
            .await?;
            anyhow::ensure!(
                updated.rows_affected() == 1,
                "workspace not found or its permanent paid plan cannot be removed"
            );
            println!("Workspace {id} uses the {plan} plan.");
            return Ok(());
        }
        _ => anyhow::bail!("usage: want-brief [-migrate-only | set-plan <workspace-id> free|paid]"),
    };
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
    let cors_origin = setting("CORS_ORIGIN", "http://127.0.0.1:5173");
    let auth = Auth::from_env(pool.clone(), &cors_origin)?;
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
        cors_origin,
        token_box,
    };
    tokio::spawn(want_brief::sources::run_scheduler(state.clone()));
    println!("want brief api on {address}");
    axum::serve(
        listener,
        router(state).into_make_service_with_connect_info::<SocketAddr>(),
    )
    .await
    .context("serve HTTP")?;
    Ok(())
}
