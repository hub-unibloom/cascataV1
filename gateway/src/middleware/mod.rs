// Middleware modules para o Pingora Gateway.
// Pipeline na ordem do SAD §B:
//   1. jwt       — JWT validation
//   2. abac      — Cedar ABAC policy check
//   3. rate_limit — Rate Limit via DragonflyDB
//   4. rls_handshake — RLS setup no upstream
// Ref: SAD §B, SRS Req-2.1.5, TASK PR-6.

pub mod jwt;
pub mod abac;
pub mod rate_limit;
pub mod rls_handshake;
