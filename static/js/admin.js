"use strict";

const $ = (id) => document.getElementById(id);
let users = [];
let currentEmail = "";

async function request(path, options = {}) {
  const response = await fetch(path, options);
  let data = {};
  try { data = await response.json(); } catch { /* empty response */ }
  if (!response.ok) throw new Error(data.error || `Request failed (${response.status})`);
  return data;
}

function notice(id, message, error = false) {
  const el = $(id);
  el.textContent = message;
  el.classList.toggle("error", error);
}

function toast(message, error = false) {
  const el = document.createElement("div");
  el.className = `toast ${error ? "err" : "ok"}`;
  el.textContent = message;
  $("toast-zone").appendChild(el);
  setTimeout(() => el.remove(), 3600);
}

async function loadOverview() {
  const data = await request("/api/admin/overview");
  $("stat-users").textContent = data.totalUsers;
  $("stat-active").textContent = data.activeUsers;
  $("stat-admins").textContent = data.adminUsers;
  $("stat-students").textContent = data.studentsGraded;
}

async function loadUsers() {
  const data = await request("/api/admin/users");
  users = data.users || [];
  renderUsers();
}

function makeButton(label, className, action, disabled = false) {
  const button = document.createElement("button");
  button.type = "button";
  button.className = className;
  button.textContent = label;
  button.disabled = disabled;
  button.addEventListener("click", action);
  return button;
}

function renderUsers() {
  const query = $("user-search").value.trim().toLowerCase();
  const rows = users.filter((user) => user.email.toLowerCase().includes(query));
  const body = $("users-body");
  body.replaceChildren();
  $("empty-state").hidden = rows.length !== 0;

  for (const user of rows) {
    const self = user.email.toLowerCase() === currentEmail.toLowerCase();
    const row = document.createElement("tr");
    const email = document.createElement("td");
    email.textContent = user.email + (self ? " (you)" : "");
    row.appendChild(email);

    const roleCell = document.createElement("td");
    const role = document.createElement("select");
    role.setAttribute("aria-label", `Role for ${user.email}`);
    for (const [value, label] of [["user", "User"], ["admin", "Admin"]]) {
      const option = document.createElement("option"); option.value = value; option.textContent = label; role.appendChild(option);
    }
    role.value = user.role;
    role.disabled = self;
    role.addEventListener("change", () => updateUser(user.email, { role: role.value }, "Role updated."));
    roleCell.appendChild(role); row.appendChild(roleCell);

    const statusCell = document.createElement("td");
    const badge = document.createElement("span");
    badge.className = `status-badge ${user.disabled ? "status-disabled" : "status-active"}`;
    badge.textContent = user.disabled ? "Disabled" : "Active";
    statusCell.appendChild(badge); row.appendChild(statusCell);

    const created = document.createElement("td");
    created.textContent = user.createdAt ? new Date(user.createdAt).toLocaleDateString() : "—";
    row.appendChild(created);

    const actions = document.createElement("td"); actions.className = "admin-actions";
    actions.appendChild(makeButton(user.disabled ? "Enable" : "Disable", "btn btn-ghost btn-small", () => updateUser(user.email, { disabled: !user.disabled }, user.disabled ? "Account enabled." : "Account disabled."), self));
    actions.appendChild(makeButton("Reset password", "btn btn-ghost btn-small", () => resetPassword(user.email)));
    actions.appendChild(makeButton("Delete", "btn btn-ghost btn-small", () => deleteUser(user.email), self));
    row.appendChild(actions);
    body.appendChild(row);
  }
}

async function updateUser(email, changes, success) {
  try {
    await request(`/api/admin/users/${encodeURIComponent(email)}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify(changes) });
    notice("table-notice", success);
    await Promise.all([loadUsers(), loadOverview()]);
  } catch (err) {
    notice("table-notice", err.message, true);
    await loadUsers();
  }
}

async function resetPassword(email) {
  const password = window.prompt(`Enter a strong temporary password for ${email}:`);
  if (password === null) return;
  try {
    await request(`/api/admin/users/${encodeURIComponent(email)}`, { method: "PATCH", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ password }) });
    toast("Password reset successfully.");
  } catch (err) { toast(err.message, true); }
}

async function deleteUser(email) {
  if (!window.confirm(`Permanently delete the account ${email}?`)) return;
  try {
    await request(`/api/admin/users/${encodeURIComponent(email)}`, { method: "DELETE" });
    notice("table-notice", "Account deleted.");
    await Promise.all([loadUsers(), loadOverview()]);
  } catch (err) { notice("table-notice", err.message, true); }
}

$("new-user-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const form = event.currentTarget;
  const button = form.querySelector("button[type=submit]");
  button.disabled = true;
  notice("create-notice", "");
  try {
    await request("/api/admin/users", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ email: form.email.value, password: form.password.value, role: form.role.value }) });
    form.reset();
    notice("create-notice", "Account created successfully.");
    await Promise.all([loadUsers(), loadOverview()]);
  } catch (err) { notice("create-notice", err.message, true); }
  finally { button.disabled = false; }
});

$("user-search").addEventListener("input", renderUsers);
$("admin-logout").addEventListener("click", async () => {
  try { await request("/api/auth/logout", { method: "POST" }); location.assign("/login"); }
  catch (err) { toast(err.message, true); }
});

(async function init() {
  try {
    const status = await request("/api/auth/status");
    currentEmail = status.email || "";
    await Promise.all([loadOverview(), loadUsers()]);
  } catch (err) { notice("table-notice", err.message, true); }
})();
