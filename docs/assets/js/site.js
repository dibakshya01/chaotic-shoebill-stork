/* chaotic-shoebill-stork landing site — vanilla JS, no deps.
   Progressive enhancement: the page is fully readable without this file.
   Respects prefers-reduced-motion (reveals resolve instantly / are no-ops). */
(function () {
  "use strict";

  var reduce =
    window.matchMedia &&
    window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  /* ---- sticky nav shadow ------------------------------------------- */
  var nav = document.querySelector(".nav");
  if (nav) {
    var onScroll = function () {
      nav.classList.toggle("is-stuck", window.scrollY > 8);
    };
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
  }

  /* ---- mobile menu toggle ------------------------------------------ */
  var toggle = document.querySelector(".nav__toggle");
  var links = document.querySelector(".nav__links");
  if (toggle && links) {
    toggle.addEventListener("click", function () {
      var open = links.classList.toggle("open");
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    });
    links.addEventListener("click", function (e) {
      if (e.target.closest("a")) {
        links.classList.remove("open");
        toggle.setAttribute("aria-expanded", "false");
      }
    });
  }

  /* ---- scroll reveals (IntersectionObserver) ----------------------- */
  // Stagger: index children so CSS delays cascade.
  document.querySelectorAll(".stagger").forEach(function (group) {
    Array.prototype.forEach.call(group.children, function (child, i) {
      child.style.setProperty("--i", i);
    });
  });

  var revealEls = document.querySelectorAll(".reveal, .stagger");

  if (reduce || !("IntersectionObserver" in window)) {
    // No motion requested (or no support): show everything immediately.
    revealEls.forEach(function (el) {
      el.classList.add("is-visible");
    });
  } else {
    var io = new IntersectionObserver(
      function (entries) {
        entries.forEach(function (entry) {
          if (entry.isIntersecting) {
            entry.target.classList.add("is-visible");
            io.unobserve(entry.target);
          }
        });
      },
      { threshold: 0.12, rootMargin: "0px 0px -8% 0px" }
    );
    revealEls.forEach(function (el) {
      io.observe(el);
    });
  }

  /* ---- copy-to-clipboard ------------------------------------------- */
  document.querySelectorAll("[data-copy]").forEach(function (btn) {
    var defaultLabel = btn.querySelector(".copy-label");
    btn.addEventListener("click", function () {
      var sel = btn.getAttribute("data-copy");
      var src = document.querySelector(sel);
      if (!src) return;
      var text = (src.getAttribute("data-copytext") || src.textContent)
        .replace(/ /g, " ")
        .trim();

      var done = function () {
        btn.classList.add("copied");
        if (defaultLabel) defaultLabel.textContent = "Copied!";
        window.setTimeout(function () {
          btn.classList.remove("copied");
          if (defaultLabel) defaultLabel.textContent = "Copy";
        }, 1900);
      };

      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(done, function () {
          legacyCopy(text, done);
        });
      } else {
        legacyCopy(text, done);
      }
    });
  });

  function legacyCopy(text, done) {
    var ta = document.createElement("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "absolute";
    ta.style.left = "-9999px";
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
      done();
    } catch (e) {
      /* ignore */
    }
    document.body.removeChild(ta);
  }

  /* ---- current year in footer -------------------------------------- */
  document.querySelectorAll("[data-year]").forEach(function (el) {
    el.textContent = new Date().getFullYear();
  });
})();
