use axum::extract::State;
use axum::response::{IntoResponse, Response};
use axum::Json;
use serde_json::json;
use sqlx::postgres::PgPoolOptions;
use sqlx::types::Json as DbJson;
use sqlx::{PgConnection, PgPool, Postgres, Transaction};
use std::future::Future;
use uuid::Uuid;

use crate::http::{database_error, AppState};

#[derive(Clone, Debug, sqlx::FromRow)]
pub struct Scope {
    pub user_id: Uuid,
    pub workspace_id: Uuid,
    pub personal_id: Uuid,
    pub kind: String,
    pub role: String,
    pub plan: String,
    pub personal_plan: String,
    pub legacy_paid: bool,
}

impl Scope {
    pub fn can_analyze(&self) -> bool {
        self.plan == "paid"
    }
}

tokio::task_local! { static ACTIVE: Scope; }

pub fn current() -> Option<Scope> {
    ACTIVE.try_with(Clone::clone).ok()
}

pub async fn in_scope<T>(scope: Scope, future: impl Future<Output = T>) -> T {
    ACTIVE.scope(scope, future).await
}

pub async fn access(
    pool: &PgPool,
    user_id: Uuid,
    requested: Option<Uuid>,
) -> Result<Scope, sqlx::Error> {
    sqlx::query_as(
        "SELECT m.user_id,w.id AS workspace_id,p.id AS personal_id,w.kind,m.role,w.plan,p.plan AS personal_plan,w.legacy_paid
         FROM workspaces p JOIN workspace_members m ON m.user_id=p.owner_id
         JOIN workspaces w ON w.id=m.workspace_id
         WHERE p.kind='personal' AND p.owner_id=$1 AND w.id=COALESCE($2,p.id)",
    ).bind(user_id).bind(requested).fetch_one(pool).await
}

// Both fresh and reused connections receive the calling task's identity before the
// first statement. A checkout outside a request or worker scope sees no domain rows.
async fn configure(connection: &mut PgConnection) -> Result<(), sqlx::Error> {
    let scope = current();
    sqlx::query("SELECT set_config('wantbrief.workspace',$1,false),set_config('wantbrief.actor',$2,false),set_config('wantbrief.backup','',false)")
        .bind(scope.as_ref().map(|s| s.workspace_id.to_string()).unwrap_or_default())
        .bind(scope.as_ref().map(|s| s.user_id.to_string()).unwrap_or_default())
        .execute(connection).await?;
    Ok(())
}

pub fn data_pool(control: &PgPool) -> PgPool {
    PgPoolOptions::new()
        .max_connections(control.options().get_max_connections())
        .after_connect(|conn, _| {
            Box::pin(async move {
                let role: String = sqlx::query_scalar("SELECT wb_runtime_role()::text")
                    .fetch_one(&mut *conn)
                    .await?;
                // The name is derived from the numeric database OID by the migration.
                if role
                    .strip_prefix("wb_data_")
                    .is_none_or(|s| s.parse::<u32>().is_err())
                {
                    return Err(sqlx::Error::Protocol("invalid data role".into()));
                }
                sqlx::query(&format!("SET ROLE {role}"))
                    .execute(&mut *conn)
                    .await?;
                configure(conn).await
            })
        })
        .before_acquire(|conn, _| {
            Box::pin(async move {
                configure(conn).await?;
                Ok(true)
            })
        })
        .connect_lazy_with((*control.connect_options()).clone())
}

pub async fn seed(
    tx: &mut Transaction<'_, Postgres>,
    workspace_id: Uuid,
    _user_id: Uuid,
) -> Result<(), sqlx::Error> {
    sqlx::query("INSERT INTO sources(workspace_id,kind,name) VALUES($1,'manual','Manual') ON CONFLICT DO NOTHING")
        .bind(workspace_id).execute(&mut **tx).await?;
    sqlx::query(
        "INSERT INTO projects(workspace_id,name,color,target_hours_day,target_hours_week)
                 SELECT $1,v.name,v.color,v.hours/7.0,v.hours
                 FROM (VALUES ('Work','#4C4CFF',30),('Life','#7C8CFF',10)) v(name,color,hours)
                 WHERE NOT EXISTS (SELECT 1 FROM projects WHERE workspace_id=$1)",
    )
    .bind(workspace_id)
    .execute(&mut **tx)
    .await?;
    Ok(())
}

pub async fn me(State(state): State<AppState>) -> Response {
    let Some(s) = current() else {
        return crate::http::error(axum::http::StatusCode::UNAUTHORIZED, "unauthorized");
    };
    match profile(state.auth.pool(), &s).await {
        Ok(data) => Json(data).into_response(),
        Err(err) => database_error(err),
    }
}

async fn profile(pool: &PgPool, s: &Scope) -> Result<serde_json::Value, sqlx::Error> {
    let email: Option<String> = sqlx::query_scalar("SELECT email FROM users WHERE id=$1")
        .bind(s.user_id)
        .fetch_one(pool)
        .await?;
    let workspaces:Vec<DbJson<serde_json::Value>>=sqlx::query_scalar("SELECT jsonb_build_object('id',w.id,'name',w.name,'kind',w.kind,'role',m.role,'plan',w.plan) FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id WHERE m.user_id=$1 ORDER BY w.kind,w.name,w.id")
        .bind(s.user_id).fetch_all(pool).await?;
    let members:Vec<DbJson<serde_json::Value>>=sqlx::query_scalar("SELECT jsonb_build_object('id',u.id,'label',coalesce(split_part(u.email,'@',1),'Installation owner'),'role',m.role) FROM users u JOIN workspace_members m ON m.user_id=u.id WHERE m.workspace_id=$1 ORDER BY u.email,u.id")
        .bind(s.workspace_id).fetch_all(pool).await?;
    let source_count: i64 =
        sqlx::query_scalar("SELECT count(*) FROM sources WHERE workspace_id=$1 AND kind<>'manual'")
            .bind(s.workspace_id)
            .fetch_one(pool)
            .await?;
    let workspace = workspaces
        .iter()
        .find(|w| w.0["id"].as_str() == Some(&s.workspace_id.to_string()))
        .ok_or(sqlx::Error::RowNotFound)?;
    Ok(
        json!({"ok":true,"user":{"id":s.user_id,"email":email},"workspaces":workspaces,"workspace":workspace,"members":members,
        "sourceCount":source_count,"personalPlan":s.personal_plan,"capabilities":{"scheduleDays":if s.plan=="paid"{28}else{1},
        "externalSourceLimit":if s.plan=="paid"{None}else{Some(2)},"canCollaborate":s.plan=="paid","canAnalyze":s.can_analyze()}}),
    )
}
