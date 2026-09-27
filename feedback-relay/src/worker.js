// The host owns GitHub App authentication; the Foundation owns the durable
// report object. Keep the Node-testable authentication adapter separate from
// the Cloudflare class export required by the production entrypoint.
export {default} from "./compat.js";
export {FeedbackReport} from "./report-store.js";
