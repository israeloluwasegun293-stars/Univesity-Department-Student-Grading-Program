/* Landing page interactions: scroll reveal + live nav state. */
"use strict";

// Reveal sections as they scroll into view.
const observer = new IntersectionObserver(
  (entries) => {
    for (const entry of entries) {
      if (entry.isIntersecting) {
        entry.target.classList.add("in-view");
        observer.unobserve(entry.target);
      }
    }
  },
  { threshold: 0.12 }
);

document
  .querySelectorAll(".landing-section, .hero-strip, .feature, .step, .scale-table-wrap, .scale-example")
  .forEach((el) => observer.observe(el));

// Smooth-scroll for in-page anchors (respects the sticky header).
document.querySelectorAll('a[href^="#"]').forEach((link) => {
  link.addEventListener("click", (e) => {
    const target = document.querySelector(link.getAttribute("href"));
    if (target) {
      e.preventDefault();
      target.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  });
});
