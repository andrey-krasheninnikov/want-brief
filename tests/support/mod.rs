use sqlx::{postgres::PgPoolOptions, PgPool};
use uuid::Uuid;

// Direct SQL in historical single-owner fixtures uses the same owner as HTTP.
pub async fn legacy_pool(url: &str) -> PgPool {
    let control = PgPool::connect(url).await.unwrap();
    want_brief::auth::Auth::new(control.clone())
        .ensure_user("sample-password")
        .await
        .unwrap();
    let (workspace, user): (Uuid, Uuid) =
        sqlx::query_as("SELECT id,owner_id FROM workspaces WHERE legacy_paid")
            .fetch_one(&control)
            .await
            .unwrap();
    control.close().await;
    PgPoolOptions::new().after_connect(move |conn,_| Box::pin(async move {
        sqlx::query("SELECT set_config('wantbrief.workspace',$1,false),set_config('wantbrief.actor',$2,false)")
            .bind(workspace.to_string()).bind(user.to_string()).execute(conn).await?;
        Ok(())
    })).connect(url).await.unwrap()
}
