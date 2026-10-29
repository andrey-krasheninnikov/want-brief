use sqlx::PgPool;
use std::path::Path;
use want_brief::migrate::migrate;

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn existing_goose_journal_is_reused() {
    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let before: i64 = sqlx::query_scalar("SELECT count(*) FROM goose_db_version")
        .fetch_one(&pool)
        .await
        .unwrap();
    assert!(before > 1);
    migrate(&url, Path::new("migrations")).await.unwrap();
    let after: i64 = sqlx::query_scalar("SELECT count(*) FROM goose_db_version")
        .fetch_one(&pool)
        .await
        .unwrap();
    let version: i64 = sqlx::query_scalar("SELECT max(version_id) FROM goose_db_version")
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(version, 48);
    assert!(after >= before);
    migrate(&url, Path::new("migrations")).await.unwrap();
    let repeated: i64 = sqlx::query_scalar("SELECT count(*) FROM goose_db_version")
        .fetch_one(&pool)
        .await
        .unwrap();
    assert_eq!(repeated, after);
}

#[tokio::test]
#[ignore = "requires an isolated TEST_DATABASE_URL"]
async fn rust_sessions_use_existing_go_format() {
    use want_brief::auth::Auth;

    let url = std::env::var("TEST_DATABASE_URL").expect("TEST_DATABASE_URL must be set");
    let database_name = url.split('?').next().unwrap().rsplit('/').next().unwrap();
    assert!(database_name.starts_with("wantbrief_issue92_"));
    let pool = PgPool::connect(&url).await.unwrap();
    let auth = Auth::new(pool.clone());
    auth.ensure_user("sample-password").await.unwrap();
    let password_hash: String =
        sqlx::query_scalar("SELECT password_hash FROM users ORDER BY created_at LIMIT 1")
            .fetch_one(&pool)
            .await
            .unwrap();
    assert!(password_hash.starts_with("$2a$10$"));
    assert!(auth.login("wrong-password").await.unwrap().is_none());
    let token = auth.login("sample-password").await.unwrap().unwrap();
    assert!(auth.authenticate(&token).await.unwrap());
    auth.logout(&token).await.unwrap();
    assert!(!auth.authenticate(&token).await.unwrap());
}
