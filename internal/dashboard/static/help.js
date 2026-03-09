// modeltap Help Page - Search and section toggle functionality
(function () {
  "use strict";

  var searchInput = document.getElementById("help-search");
  var searchCount = document.getElementById("help-search-count");
  var noResults = document.getElementById("help-no-results");
  var sectionsContainer = document.getElementById("help-sections");
  var sections = document.querySelectorAll("[data-help-section]");

  // === Section collapse/expand ===
  function initSectionToggles() {
    sections.forEach(function (section) {
      var title = section.querySelector(".help-section-title");
      if (!title) return;

      title.addEventListener("click", function () {
        section.classList.toggle("collapsed");
      });
    });
  }

  // === Search/filter ===
  function filterSections(query) {
    var q = query.trim().toLowerCase();

    if (!q) {
      // Show all sections, restore collapsed state
      sections.forEach(function (section) {
        section.classList.remove("hidden");
      });
      noResults.hidden = true;
      searchCount.textContent = "";
      return;
    }

    var visibleCount = 0;

    sections.forEach(function (section) {
      var text = section.textContent.toLowerCase();
      var matches = text.indexOf(q) !== -1;

      if (matches) {
        section.classList.remove("hidden");
        // Expand matching sections so content is visible
        section.classList.remove("collapsed");
        visibleCount++;
      } else {
        section.classList.add("hidden");
      }
    });

    noResults.hidden = visibleCount > 0;

    if (visibleCount > 0) {
      searchCount.textContent = visibleCount + " of " + sections.length + " sections";
    } else {
      searchCount.textContent = "0 results";
    }
  }

  // Debounce helper to avoid filtering on every keystroke
  function debounce(fn, delay) {
    var timer = null;
    return function () {
      var args = arguments;
      var context = this;
      if (timer) clearTimeout(timer);
      timer = setTimeout(function () {
        fn.apply(context, args);
      }, delay);
    };
  }

  var debouncedFilter = debounce(function () {
    filterSections(searchInput.value);
  }, 150);

  // === Init ===
  function init() {
    initSectionToggles();

    searchInput.addEventListener("input", debouncedFilter);

    // Support clearing with Escape key
    searchInput.addEventListener("keydown", function (e) {
      if (e.key === "Escape") {
        searchInput.value = "";
        filterSections("");
        searchInput.blur();
      }
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
