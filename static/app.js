/* =====================================================================
   ClassEdge — frontend logic.
   Talks to the Go backend through the dedicated mux's /api/* routes.
   ===================================================================== */

"use strict";

const $ = (id) => document.getElementById(id);

// ---------- grade rules mirrored from grades.go (5.0 scale) ----------
function gradeFor(avg) {
  if (avg >= 70) return "A";
  if (avg >= 60) return "B";
  if (avg >= 50) return "C";
  if (avg >= 45) return "D";
  if (avg >= 40) return "E";
  return "F";
}

const GRADE_COLORS = {
  A: "#34e5a1", B: "#00e5ff", C: "#ffcf5c",
  D: "#ff8c42", E: "#ff8c42", F: "#ff6b81",
};

const VERDICTS = {
  A: "Outstanding — first class territory! 🎓",
  B: "Very good — a strong, steady performance.",
  C: "Good — solid work with room to climb.",
  D: "Fair — the fundamentals are there. Push harder!",
  E: "Pass — narrowly across the line. Grind time.",
  F: "Failed — every legend starts with a comeback story.",
};

// ---------- API helpers ----------
async function api(path, options) {
  const res = await fetch(path, options);
  let data = null;
  try { data = await res.json(); } catch { /* non-JSON error body */ }
  if (!res.ok) {
    const msg = (data && data.error) ? data.error : `Request failed (${res.status})`;
    throw new Error(msg);
  }
  return data;
}

// ---------- toasts ----------
function toast(message, kind = "ok") {
  const zone = $("toast-zone");
  const el = document.createElement("div");
  el.className = `toast ${kind}`;
  el.textContent = message;
  zone.appendChild(el);
  setTimeout(() => {
    el.style.opacity = "0";
    el.style.transition = "opacity 0.4s";
    setTimeout(() => el.remove(), 400);
  }, 3400);
}

// ---------- confetti (for the A-grade moment) ----------
function confettiBurst(count = 90) {
  const colors = ["#7c5cff", "#00e5ff", "#34e5a1", "#ffcf5c", "#ff6b81", "#ff8c42"];
  for (let i = 0; i < count; i++) {
    const bit = document.createElement("div");
    bit.className = "confetti";
    bit.style.left = Math.random() * 100 + "vw";
    bit.style.background = colors[(Math.random() * colors.length) | 0];
    bit.style.animationDuration = 2.2 + Math.random() * 1.8 + "s";
    bit.style.animationDelay = Math.random() * 0.4 + "s";
    bit.style.transform = `rotate(${Math.random() * 360}deg)`;
    document.body.appendChild(bit);
    setTimeout(() => bit.remove(), 4800);
  }
}

// ---------- form / preview ----------
const scoreInputs = Array.from(document.querySelectorAll(".score-input"));

function collectScores() {
  const marks = [];
  for (const input of scoreInputs) {
    const raw = input.value.trim();
    if (raw === "") continue;
    const n = Number(raw);
    if (!Number.isFinite(n)) return null;
    marks.push(n);
  }
  return marks.length ? marks : null;
}

function updatePreview() {
  const gradeEl = $("preview-grade");
  const detailEl = $("preview-detail");
  const marks = collectScores();

  if (!marks) {
    gradeEl.textContent = "—";
    gradeEl.style.color = "var(--muted)";
    detailEl.textContent = "enter scores to preview";
    return;
  }

  const bad = marks.some((m) => m < 0 || m > 100);
  if (bad) {
    gradeEl.textContent = "?!";
    gradeEl.style.color = "var(--red)";
    detailEl.textContent = "scores must be between 0 and 100";
    return;
  }

  const avg = marks.reduce((a, b) => a + b, 0) / marks.length;
  const grade = gradeFor(avg);
  const gpa = { A: 5, B: 4, C: 3, D: 2, E: 1, F: 0 }[grade];
  gradeEl.textContent = grade;
  gradeEl.style.color = GRADE_COLORS[grade];
  detailEl.textContent = `avg ${avg.toFixed(2)} → projected GPA ${gpa.toFixed(1)}`;
}

scoreInputs.forEach((inp) => inp.addEventListener("input", updatePreview));

// ---------- submit ----------
$("grade-form").addEventListener("submit", async (event) => {
  event.preventDefault();

  const name = $("student-name").value.trim();
  const matrikNo = $("student-matric").value.trim();
  const marks = collectScores();
  const errEl = $("form-error");
  errEl.hidden = true;

  if (!name || !matrikNo) {
    errEl.textContent = "Name and matric number are required.";
    errEl.hidden = false;
    return;
  }
  if (!marks) {
    errEl.textContent = "Enter at least one course score.";
    errEl.hidden = false;
    return;
  }
  if (marks.some((m) => m < 0 || m > 100)) {
    errEl.textContent = "Every score must be a number between 0 and 100.";
    errEl.hidden = false;
    return;
  }

  const btn = $("submit-btn");
  btn.disabled = true;
  const face = btn.querySelector(".btn-face");
  const original = face.textContent;
  face.textContent = "Crunching numbers…";

  try {
    const data = await api("/api/grade", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, matrikNo, marks }),
    });

    const graded = data.students[data.students.length - 1];
    showReveal(graded);
    renderDashboard(data);

    if (graded.GradeLeter === "A") confettiBurst();
    toast(`${graded.Name} graded: ${graded.GradeLeter} · GPA ${graded.GPA.toFixed(1)}`, "ok");
  } catch (err) {
    errEl.textContent = err.message;
    errEl.hidden = false;
    toast(err.message, "err");
  } finally {
    btn.disabled = false;
    face.textContent = original;
  }
});

// ---------- reveal panel ----------
function showReveal(student) {
  $("reveal-empty").hidden = true;
  $("reveal-body").hidden = false;

  const medal = $("grade-medal");
  medal.className = "grade-medal";
  void medal.offsetWidth; // restart the pop animation
  medal.classList.add("pop", `g-${student.GradeLeter}`);

  $("reveal-grade").textContent = student.GradeLeter;
  $("reveal-name").textContent = student.Name;
  $("reveal-matric").textContent = student.MatrikNo;
  $("reveal-verdict").textContent = VERDICTS[student.GradeLeter] || "";
  $("reveal-gpa").textContent = student.GPA.toFixed(2);
  $("reveal-avg").textContent = student.AverageMark.toFixed(2);
  $("reveal-total").textContent = student.TotalMarks.toFixed(2);

  // Count used course slots: all leading scores up to the last non-empty entry.
  let courseCount = 0;
  for (let i = 0; i < student.Marks.length; i++) {
    if (student.Marks[i] > 0 || i === 0) courseCount = i + 1;
  }
  $("reveal-courses").textContent = String(courseCount);

  // GPA ring animation (r = 52 → circumference ≈ 326.7)
  const ring = $("ring-fg");
  const frac = Math.max(0, Math.min(1, student.GPA / 5));
  ring.style.strokeDashoffset = String(326.7 * (1 - frac));
  ring.style.stroke = GRADE_COLORS[student.GradeLeter] || "#00e5ff";

  // course chips
  const chips = $("course-chips");
  chips.innerHTML = "";
  student.Marks.forEach((m, i) => {
    if (m <= 0 && i > 0) return; // hide unused trailing slots
    const chip = document.createElement("span");
    chip.className = "course-chip";
    chip.textContent = `C${i + 1}: ${m}`;
    chip.style.borderColor = GRADE_COLORS[gradeFor(m)] + "66";
    chips.appendChild(chip);
  });
}

// ---------- dashboard: metrics + leaderboard ----------
function renderDashboard(data) {
  const s = data.summary;

  $("stat-count").textContent = String(data.totalCount);
  $("stat-avg-gpa").textContent = s.averageGPA.toFixed(2);
  $("stat-high-avg").textContent = s.highestAvg.toFixed(2);
  $("stat-low-avg").textContent = s.lowestAvg.toFixed(2);

  renderDistribution(s.gradeCounts || {});
  renderLeaderboard(data.students);
}

function renderDistribution(counts) {
  const wrap = $("dist-bars");
  wrap.innerHTML = "";
  const max = Math.max(1, ...Object.values(counts));

  for (const grade of ["A", "B", "C", "D", "E", "F"]) {
    const n = counts[grade] || 0;
    const col = document.createElement("div");
    col.className = "dist-bar";
    col.innerHTML = `
      <span class="dist-count">${n}</span>
      <div class="dist-fill" style="height:${(n / max) * 82}%; background:linear-gradient(180deg, ${GRADE_COLORS[grade]}, ${GRADE_COLORS[grade]}44);"></div>
      <span class="dist-grade" style="color:${GRADE_COLORS[grade]}">${grade}</span>`;
    wrap.appendChild(col);
  }
}

function renderLeaderboard(students) {
  const body = $("roster-body");
  body.innerHTML = "";

  if (!students.length) {
    body.innerHTML = `<tr class="empty-row"><td colspan="7">No students yet — be the first to reveal a result.</td></tr>`;
    return;
  }

  // newest first
  const ordered = [...students].reverse();
  ordered.forEach((s, idx) => {
    const tr = document.createElement("tr");
    if (idx === 0) tr.className = "row-new";
    tr.innerHTML = `
      <td>${idx + 1}</td>
      <td>${escapeHtml(s.Name)}</td>
      <td class="mono">${escapeHtml(s.MatrikNo)}</td>
      <td>${s.TotalMarks.toFixed(1)}</td>
      <td>${s.AverageMark.toFixed(2)}</td>
      <td><span class="pill pill-${s.GradeLeter}">${s.GradeLeter}</span></td>
      <td class="mono">${s.GPA.toFixed(2)}</td>`;
    body.appendChild(tr);
  });
}

// Security: never inject server strings as raw HTML.
function escapeHtml(str) {
  return String(str)
    .replaceAll("&", "&amp;").replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;").replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

// ---------- reset ----------
$("reset-btn").addEventListener("click", async () => {
  try {
    await api("/api/reset", { method: "POST" });
    const data = await api("/api/results");
    renderDashboard(data);
    $("reveal-body").hidden = true;
    $("reveal-empty").hidden = false;
    toast("Roster cleared — fresh start!", "ok");
  } catch (err) {
    toast(err.message, "err");
  }
});

// ---------- health check on load ----------
(async function checkHealth() {
  const chip = $("server-chip");
  try {
    const data = await api("/api/health");
    chip.classList.add("online");
    $("server-status").textContent = "backend online";
    $("go-version").textContent = data.go || "";
  } catch {
    chip.classList.add("offline");
    $("server-status").textContent = "backend unreachable";
  }
})();
