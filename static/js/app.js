/* =====================================================================
   ClassEdge — dashboard logic.
   Batch wizard: pick how many students → per-student forms with live
   per-course grade chips → submit to /api/grade/batch → formatted report.
   Talks to the Go backend through the dedicated mux's /api/* routes.
   ===================================================================== */

"use strict";

const $ = (id) => document.getElementById(id);

// ---------- grade rules mirrored from grading/grading.go (5.0 scale) ----------
function bandFor(score) {
  if (score >= 70) return { letter: "A", point: 5, label: "Excellent" };
  if (score >= 60) return { letter: "B", point: 4, label: "Very Good" };
  if (score >= 50) return { letter: "C", point: 3, label: "Credit" };
  if (score >= 45) return { letter: "D", point: 2, label: "Pass" };
  if (score >= 40) return { letter: "E", point: 1, label: "Low Pass" };
  return { letter: "F", point: 0, label: "Fail" };
}

const GRADE_COLORS = {
  A: "#34e5a1", B: "#00e5ff", C: "#ffcf5c",
  D: "#ff8c42", E: "#ff8c42", F: "#ff6b81",
};

const VERDICTS = {
  A: "Excellent — first class territory! 🎓",
  B: "Very Good — a strong, steady performance.",
  C: "Credit — solid work with room to climb.",
  D: "Pass — the fundamentals are there. Push harder!",
  E: "Low Pass — narrowly across the line. Grind time.",
  F: "Fail — every legend starts with a comeback story.",
};

// ---------- API helper ----------
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

// ---------- escape helper ----------
function escapeHtml(str) {
  return String(str)
    .replaceAll("&", "&amp;").replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;").replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

/* =====================================================================
   STEP 1 — batch size
   ===================================================================== */
let batchCount = 1;

function bindCountButtons() {
  document.querySelectorAll(".count-btn").forEach((btn) => {
    btn.addEventListener("click", () => {
      $("student-count").value = btn.dataset.count;
      document.querySelectorAll(".count-btn").forEach((b) => b.classList.remove("active"));
      btn.classList.add("active");
    });
  });
  // highlight the default
  document.querySelector('.count-btn[data-count="1"]').classList.add("active");
}

function startBatch() {
  const n = parseInt($("student-count").value, 10);
  if (!Number.isInteger(n) || n < 1 || n > 100) {
    toast("Enter a student count between 1 and 100.", "err");
    return;
  }
  batchCount = n;
  buildStudentForms(n);
  $("batch-box").hidden = true;
  $("grade-form").hidden = false;
  $("grade-form").scrollIntoView({ behavior: "smooth", block: "start" });
}

function buildStudentForms(n) {
  const wrap = $("student-forms");
  wrap.innerHTML = "";
  for (let i = 0; i < n; i++) {
    wrap.insertAdjacentHTML("beforeend", `
      <div class="sform" data-index="${i}">
        <div class="sform-head">
          <span class="sform-title">Student ${i + 1}</span>
          <span class="sform-live" data-live="${i}">—</span>
        </div>
        <div class="field">
          <label>Student full name</label>
          <input type="text" class="st-name" maxlength="120" placeholder="e.g. Ada Lovelace" />
        </div>
        <div class="field">
          <label>Matriculation number</label>
          <input type="text" class="st-matric" maxlength="40" placeholder="e.g. CSC/2026/0142" />
        </div>
        <div class="field">
          <label>Course scores (0–100) — leave later ones blank if fewer courses</label>
          <div class="score-grid">
            ${[1, 2, 3, 4, 5].map((c) => `
              <div class="score-cell">
                <span class="score-tag">C${c}</span>
                <input type="number" class="score-input" min="0" max="100" step="0.5" placeholder="—" aria-label="Student ${i + 1} course ${c} score" />
              </div>`).join("")}
          </div>
        </div>
      </div>`);
  }

  // Live per-course grade chips as scores are typed.
  wrap.querySelectorAll(".score-input").forEach((inp) => {
    inp.addEventListener("input", () => {
      const box = inp.closest(".sform");
      const idx = box.dataset.index;
      const live = box.querySelector(`[data-live="${idx}"]`);
      const scores = readScores(box);
      if (!scores.length) {
        live.textContent = "—";
        live.style.color = "";
        return;
      }
      if (scores.some((s) => s < 0 || s > 100)) {
        live.textContent = "?!";
        live.style.color = GRADE_COLORS.F;
        return;
      }
      const gpa = scores.reduce((acc, s) => acc + bandFor(s).point, 0) / scores.length;
      const letters = scores.map((s) => bandFor(s).letter).join("·");
      live.textContent = `${letters} → ${gpa.toFixed(2)}`;
      const avgScore = scores.reduce((a, s) => a + s, 0) / scores.length;
      live.style.color = GRADE_COLORS[bandFor(avgScore).letter];
    });
  });
}

function readScores(formBox) {
  const marks = [];
  for (const input of formBox.querySelectorAll(".score-input")) {
    const raw = input.value.trim();
    if (raw === "") continue;
    const n = Number(raw);
    if (!Number.isFinite(n)) return null;
    marks.push(n);
  }
  return marks;
}

/* =====================================================================
   STEP 2 — submit the batch
   ===================================================================== */
function bindSubmit() {
  $("grade-form").addEventListener("submit", async (event) => {
    event.preventDefault();
    const errEl = $("form-error");
    errEl.hidden = true;

    const formBoxes = Array.from(document.querySelectorAll(".sform"));
    const students = [];
    const problems = [];

    formBoxes.forEach((box, i) => {
      const name = box.querySelector(".st-name").value.trim();
      const matrikNo = box.querySelector(".st-matric").value.trim();
      const marks = readScores(box) || [];

      if (!name) problems.push(`Student ${i + 1}: name is required.`);
      else if (!matrikNo) problems.push(`Student ${i + 1}: matric number is required.`);
      else if (!marks.length) problems.push(`Student ${i + 1}: enter at least one course score.`);
      else if (marks.some((m) => m < 0 || m > 100)) problems.push(`Student ${i + 1}: every score must be 0–100.`);

      students.push({ name, matrikNo, marks });
    });

    if (problems.length) {
      errEl.textContent = problems[0];
      errEl.hidden = false;
      return;
    }

    const btn = $("submit-btn");
    btn.disabled = true;
    const face = btn.querySelector(".btn-face");
    const original = face.textContent;
    face.textContent = "Crunching numbers…";

    try {
      const data = await api("/api/grade/batch", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ students }),
      });

      renderDashboard(data);

      const best = [...data.students].sort((a, b) => b.GPA - a.GPA)[0];
      if (best && best.GradeLeter === "A") confettiBurst();

      const gradedNow = data.students.slice(-students.length);
      toast(`Graded ${gradedNow.length} student(s). Class avg CGPA: ${data.summary.averageGPA.toFixed(2)}`, "ok");
    } catch (err) {
      errEl.textContent = err.message;
      errEl.hidden = false;
      toast(err.message, "err");
    } finally {
      btn.disabled = false;
      face.textContent = original;
    }
  });
}

/* =====================================================================
   STEP 3 — dashboard rendering
   ===================================================================== */
function renderDashboard(data) {
  const s = data.summary;

  $("report-section").hidden = false;
  $("metrics-section").hidden = false;
  $("table-section").hidden = false;

  $("report-pre").textContent = data.report;

  $("stat-count").textContent = String(data.totalCount);
  $("stat-avg-gpa").textContent = s.averageGPA.toFixed(2);
  $("stat-high-avg").textContent = s.highestAvg.toFixed(2);
  $("stat-low-avg").textContent = s.lowestAvg.toFixed(2);

  renderDistribution(s.gradeCounts || {});
  renderLeaderboard(data.students);

  // Reveal the top student of the batch in the Result Reveal panel.
  if (data.students.length) {
    showReveal(data.students[data.students.length - 1]);
  }

  $("report-section").scrollIntoView({ behavior: "smooth", block: "start" });
}

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
  $("reveal-verdict").textContent = VERDICTS[student.GradeLeter] || student.GradeLabel || "";
  $("reveal-gpa").textContent = student.GPA.toFixed(2);
  $("reveal-avg").textContent = student.AverageMark.toFixed(2);
  $("reveal-total").textContent = student.TotalMarks.toFixed(2);

  $("reveal-courses").textContent = String(student.Marks.length);

  const ring = $("ring-fg");
  const frac = Math.max(0, Math.min(1, student.GPA / 5));
  ring.style.strokeDashoffset = String(326.7 * (1 - frac));
  ring.style.stroke = GRADE_COLORS[student.GradeLeter] || "#00e5ff";

  const chips = $("course-chips");
  chips.innerHTML = "";
  student.CourseGrades.forEach((cg) => {
    const chip = document.createElement("span");
    chip.className = "course-chip";
    chip.textContent = `${cg.Course}: ${cg.Score} → ${cg.Letter} (${cg.Point})`;
    chip.style.borderColor = GRADE_COLORS[cg.Letter] + "66";
    chips.appendChild(chip);
  });
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

/* =====================================================================
   Wiring
   ===================================================================== */
function bindReset() {
  $("reset-btn").addEventListener("click", async () => {
    try {
      await api("/api/reset", { method: "POST" });
      $("report-section").hidden = true;
      $("metrics-section").hidden = true;
      $("table-section").hidden = true;
      $("reveal-body").hidden = true;
      $("reveal-empty").hidden = false;
      toast("Roster cleared — fresh start!", "ok");
    } catch (err) {
      toast(err.message, "err");
    }
  });
}

function bindReportActions() {
  $("copy-report").addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText($("report-pre").textContent);
      toast("Report copied to clipboard.", "ok");
    } catch {
      toast("Could not copy — select the text manually.", "err");
    }
  });
  $("print-report").addEventListener("click", () => window.print());
}

function bindBackButton() {
  $("back-btn").addEventListener("click", () => {
    $("grade-form").hidden = true;
    $("batch-box").hidden = false;
  });
}

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

bindCountButtons();
bindSubmit();
bindReset();
bindReportActions();
bindBackButton();
