// Mejora progresiva de la lista de tareas: alta, edición en la tarjeta, completar
// y eliminar con Deshacer sin recargar la página. Sin JavaScript, los mismos
// formularios funcionan con recarga (respuestas 303 del servidor).
(function () {
  "use strict";

  var TOAST_MS = 8000;
  var editing = null; // tarjeta en modo edición
  var toast = null; // aviso visible: {el, timer, remaining, startedAt, undo, returnFocus}
  var restoring = false;

  // ---------------------------------------------------------------- utilidades

  function csrfToken() {
    var meta = document.querySelector('meta[name="csrf-token"]');
    return meta ? meta.getAttribute("content") : "";
  }

  function reduceMotion() {
    return window.matchMedia && window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  }

  // Añade la clase de animación y resuelve cuando termina (o a los 400 ms).
  function animateOut(el, cls) {
    return new Promise(function (resolve) {
      if (reduceMotion()) {
        resolve();
        return;
      }
      var done = false;
      function finish() {
        if (done) return;
        done = true;
        el.removeEventListener("animationend", onEnd);
        resolve();
      }
      function onEnd(event) {
        if (event.target === el) finish();
      }
      el.addEventListener("animationend", onEnd);
      el.classList.add(cls);
      window.setTimeout(finish, 400);
    });
  }

  function animateIn(el, cls) {
    if (reduceMotion()) return;
    el.classList.add(cls);
    function clear() {
      el.classList.remove(cls);
    }
    el.addEventListener("animationend", function onEnd(event) {
      if (event.target !== el) return;
      el.removeEventListener("animationend", onEnd);
      clear();
    });
    window.setTimeout(clear, 400);
  }

  // Convierte un fragmento HTML del servidor (ya escapado por Plush) en un elemento.
  function toElement(html) {
    var template = document.createElement("template");
    template.innerHTML = html.trim();
    return template.content.firstElementChild;
  }

  function send(url, body) {
    return fetch(url, {
      method: "POST",
      body: new URLSearchParams(body),
      credentials: "same-origin",
      headers: {
        "X-Requested-With": "XMLHttpRequest",
        "X-CSRF-Token": csrfToken(),
        Accept: "text/html"
      }
    }).then(function (res) {
      return res.text().then(function (html) {
        return { status: res.status, html: html };
      });
    });
  }

  function announce(text) {
    var live = document.querySelector("[data-live]");
    if (!live) return;
    live.textContent = "";
    window.setTimeout(function () {
      live.textContent = text;
    }, 50);
  }

  // Recurre al envío clásico (sin JS). No dispara el evento submit.
  function fallback(form) {
    HTMLFormElement.prototype.submit.call(form);
  }

  function submitForm(form) {
    if (typeof form.requestSubmit === "function") {
      form.requestSubmit();
    } else {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    }
  }

  function grid() {
    return document.querySelector("[data-card-grid]");
  }

  function updateCount() {
    var list = grid();
    var n = list ? list.querySelectorAll("[data-task-card]").length : 0;
    var count = document.querySelector("[data-task-count]");
    if (count) {
      count.textContent = n + " " + (n === 1 ? "tarea en tu lista" : "tareas en tu lista");
    }
    var empty = document.querySelector("[data-empty-state]");
    if (empty) empty.hidden = n > 0;
  }

  function setColorClass(el, color) {
    Array.prototype.slice.call(el.classList).forEach(function (cls) {
      if (cls.indexOf("color-") === 0) el.classList.remove(cls);
    });
    el.classList.add("color-" + color);
  }

  function currentColorClass(el) {
    var found = "";
    el.classList.forEach(function (cls) {
      if (cls.indexOf("color-") === 0) found = cls;
    });
    return found;
  }

  function autosize(textarea) {
    textarea.style.height = "auto";
    textarea.style.height = textarea.scrollHeight + "px";
  }

  function setBusy(card, busy) {
    if (busy) {
      card.setAttribute("aria-busy", "true");
    } else {
      card.removeAttribute("aria-busy");
    }
    card.querySelectorAll("button").forEach(function (button) {
      button.disabled = busy;
    });
  }

  function isTextField(el) {
    return el && (el.tagName === "INPUT" || el.tagName === "TEXTAREA") && el.type !== "radio";
  }

  // ------------------------------------------------------------------ composer

  function composerIsEmpty(form) {
    return form.elements.title.value.trim() === "" && form.elements.description.value.trim() === "";
  }

  function initComposer(form) {
    if (!form || !document.documentElement.classList.contains("js")) return;
    if (!form.querySelector(".field-error") && composerIsEmpty(form)) {
      form.classList.add("is-collapsed");
    }
  }

  function expandComposer(form) {
    form.classList.remove("is-collapsed");
  }

  function collapseComposer(form) {
    form.classList.add("is-collapsed");
  }

  // Devuelve el foco al título sin volver a expandir el composer.
  function focusComposerTitle(form) {
    form.dataset.keepCollapsed = "true";
    form.elements.title.focus();
    delete form.dataset.keepCollapsed;
  }

  function resetComposer(form) {
    form.elements.title.value = "";
    form.elements.description.value = "";
    form.elements.description.style.height = "";
    var standard = form.querySelector('input[name="color"][value="default"]');
    if (standard) standard.checked = true;
    setColorClass(form, "default");
    var error = form.querySelector(".field-error");
    if (error) error.remove();
    form.elements.title.removeAttribute("aria-invalid");
    form.elements.title.removeAttribute("aria-describedby");
  }

  function submitComposer(form) {
    if (form.dataset.busy) return;
    form.dataset.busy = "true";
    send(form.action, new FormData(form))
      .then(function (res) {
        delete form.dataset.busy;
        if (res.status === 201) {
          var card = toElement(res.html);
          grid().appendChild(card);
          animateIn(card, "is-entering");
          card.scrollIntoView({ block: "nearest" });
          resetComposer(form);
          form.elements.title.focus();
          announce("Tarea creada.");
          updateCount();
        } else if (res.status === 422) {
          var next = toElement(res.html);
          form.replaceWith(next);
          var title = next.querySelector("#task-title");
          if (title) title.focus();
        } else {
          fallback(form);
        }
      })
      .catch(function () {
        delete form.dataset.busy;
        fallback(form);
      });
  }

  // ------------------------------------------------------- edición en tarjeta

  function serialize(form) {
    return new URLSearchParams(new FormData(form)).toString();
  }

  function isDirty(card) {
    var form = card.querySelector("[data-card-edit]");
    return form && serialize(form) !== card.dataset.snapshot;
  }

  function openEdit(card) {
    if (editing && editing !== card) {
      if (isDirty(editing)) {
        submitForm(editing.querySelector("[data-card-edit]"));
      } else {
        closeEdit(editing, { restore: false, focus: false });
      }
    }

    var form = card.querySelector("[data-card-edit]");
    var view = card.querySelector("[data-card-view]");
    var title = form.elements.title;

    if (editing !== card) {
      card.dataset.snapshot = serialize(form);
      card.dataset.colorOriginal = currentColorClass(card).replace("color-", "");
      form.hidden = false;
      view.hidden = true;
      card.classList.add("is-editing");
      editing = card;
      autosize(form.elements.description);
    }

    title.focus();
    var end = title.value.length;
    title.setSelectionRange(end, end);
  }

  function closeEdit(card, options) {
    var form = card.querySelector("[data-card-edit]");
    var view = card.querySelector("[data-card-view]");
    if (options.restore) {
      form.reset();
      if (card.dataset.colorOriginal) setColorClass(card, card.dataset.colorOriginal);
    }
    form.hidden = true;
    view.hidden = false;
    card.classList.remove("is-editing");
    if (editing === card) editing = null;
    if (options.focus) {
      var open = card.querySelector("[data-edit-open]");
      if (open) open.focus();
    }
  }

  function submitEdit(form) {
    var card = form.closest("[data-task-card]");
    if (card.getAttribute("aria-busy") === "true") return;
    var body = new FormData(form);
    setBusy(card, true);
    send(form.action, body)
      .then(function (res) {
        if (res.status === 200 || res.status === 422) {
          var hadFocus = card.contains(document.activeElement) || document.activeElement === document.body;
          var next = toElement(res.html);
          card.replaceWith(next);
          if (editing === card) editing = null;

          if (res.status === 200) {
            if (hadFocus) {
              var open = next.querySelector("[data-edit-open]");
              if (open) open.focus();
            }
            announce("Tarea actualizada.");
          } else {
            var nextForm = next.querySelector("[data-card-edit]");
            next.dataset.colorOriginal = currentColorClass(next).replace("color-", "");
            next.dataset.snapshot = serialize(nextForm);
            if (editing && editing !== next) {
              closeEdit(editing, { restore: false, focus: false });
            }
            editing = next;
            nextForm.elements.title.focus();
          }
        } else {
          setBusy(card, false);
          fallback(form);
        }
      })
      .catch(function () {
        setBusy(card, false);
        fallback(form);
      });
  }

  // ------------------------------------------------------------------- toggle

  function submitToggle(form) {
    var card = form.closest("[data-task-card]");
    if (card.getAttribute("aria-busy") === "true") return;
    var body = new FormData(form);
    card.classList.toggle("is-completed");
    setBusy(card, true);
    send(form.action, body)
      .then(function (res) {
        if (res.status !== 200) throw new Error("toggle " + res.status);
        var hadFocus = card.contains(document.activeElement) || document.activeElement === document.body;
        var next = toElement(res.html);
        if (editing === card) {
          // Conserva la edición en curso: solo cambia el estado de completada.
          card.querySelectorAll("[data-card-edit] button").forEach(function (button) {
            button.disabled = false;
          });
          next.querySelector("[data-card-edit]").replaceWith(card.querySelector("[data-card-edit]"));
          next.querySelector("[data-card-view]").hidden = true;
          setColorClass(next, currentColorClass(card).replace("color-", ""));
          next.classList.add("is-editing");
          next.dataset.snapshot = card.dataset.snapshot;
          next.dataset.colorOriginal = card.dataset.colorOriginal;
          editing = next;
        }
        card.replaceWith(next);
        if (hadFocus) {
          var button = next.querySelector("[data-task-toggle] button");
          if (button) button.focus();
        }
      })
      .catch(function () {
        card.classList.toggle("is-completed");
        setBusy(card, false);
        fallback(form);
      });
  }

  // ------------------------------------------------------ eliminar y deshacer

  function focusTargetAfterRemoval(card) {
    var sibling = card.nextElementSibling || card.previousElementSibling;
    if (sibling) {
      return sibling.querySelector("[data-task-delete] button") || sibling.querySelector("[data-edit-open]");
    }
    return document.getElementById("task-title");
  }

  function submitDelete(form) {
    var card = form.closest("[data-task-card]");
    if (card.getAttribute("aria-busy") === "true") return;
    var id = card.dataset.taskId;
    var next = card.nextElementSibling;
    var nextId = next ? next.dataset.taskId : null;
    var target = focusTargetAfterRemoval(card);
    var body = new FormData(form);
    setBusy(card, true);
    send(form.action, body)
      .then(function (res) {
        if (res.status !== 204) {
          setBusy(card, false);
          fallback(form);
          return;
        }
        if (editing === card) editing = null;
        return animateOut(card, "is-leaving").then(function () {
          card.remove();
          if (target && document.contains(target)) target.focus();
          updateCount();
          showToast("Tarea eliminada.", { id: id, nextId: nextId }, target);
        });
      })
      .catch(function () {
        setBusy(card, false);
        fallback(form);
      });
  }

  function startToastTimer() {
    if (!toast) return;
    window.clearTimeout(toast.timer);
    toast.startedAt = Date.now();
    toast.timer = window.setTimeout(function () {
      dismissToast(false);
    }, toast.remaining);
  }

  function pauseToastTimer() {
    if (!toast || !toast.timer) return;
    window.clearTimeout(toast.timer);
    toast.timer = null;
    toast.remaining = Math.max(1000, toast.remaining - (Date.now() - toast.startedAt));
  }

  function dismissToast(immediate) {
    if (!toast) return;
    var current = toast;
    toast = null;
    window.clearTimeout(current.timer);
    var hadFocus = current.el.contains(document.activeElement);
    if (hadFocus) {
      var fallbackFocus = current.returnFocus && document.contains(current.returnFocus)
        ? current.returnFocus
        : document.getElementById("task-title");
      if (fallbackFocus) fallbackFocus.focus();
    }
    if (immediate) {
      current.el.remove();
      return;
    }
    animateOut(current.el, "is-leaving").then(function () {
      current.el.remove();
    });
  }

  function showToast(message, undoData, returnFocus) {
    var region = document.querySelector("[data-toast-region]");
    var template = document.querySelector("[data-toast-template]");
    if (!region || !template) return;

    dismissToast(true);

    var el = template.content.firstElementChild.cloneNode(true);
    el.querySelector("[data-toast-message]").textContent = message;
    var undoButton = el.querySelector("[data-toast-undo]");
    if (undoData) {
      undoButton.addEventListener("click", undo);
    } else {
      undoButton.remove();
    }
    el.querySelector("[data-toast-close]").addEventListener("click", function () {
      dismissToast(false);
    });
    el.addEventListener("mouseenter", pauseToastTimer);
    el.addEventListener("mouseleave", startToastTimer);
    el.addEventListener("focusin", pauseToastTimer);
    el.addEventListener("focusout", function (event) {
      if (!el.contains(event.relatedTarget)) startToastTimer();
    });
    el.addEventListener("keydown", function (event) {
      if (event.key === "Escape") {
        event.preventDefault();
        dismissToast(false);
      }
    });

    region.appendChild(el);
    toast = { el: el, timer: null, remaining: TOAST_MS, startedAt: 0, undo: undoData, returnFocus: returnFocus };
    startToastTimer();
  }

  function undo() {
    if (!toast || !toast.undo || restoring) return;
    var data = toast.undo;
    restoring = true;
    send("/tasks/" + encodeURIComponent(data.id) + "/restore", { authenticity_token: csrfToken() })
      .then(function (res) {
        restoring = false;
        if (res.status === 200) {
          var card = toElement(res.html);
          var list = grid();
          var ref = data.nextId ? document.getElementById("task-" + data.nextId) : null;
          if (ref && ref.parentNode === list) {
            list.insertBefore(card, ref);
          } else {
            list.appendChild(card);
          }
          animateIn(card, "is-entering");
          var open = card.querySelector("[data-edit-open]");
          dismissToast(true);
          if (open) open.focus();
          announce("Tarea restaurada.");
          updateCount();
        } else if (res.status === 404) {
          showToast("No se pudo deshacer: la tarea ya no está disponible.", null, toast && toast.returnFocus);
        } else {
          showToast("No se pudo deshacer la eliminación.", null, toast && toast.returnFocus);
        }
      })
      .catch(function () {
        restoring = false;
        showToast("No se pudo deshacer la eliminación.", null, toast && toast.returnFocus);
      });
  }

  // ------------------------------------------------------------------ eventos

  document.addEventListener("submit", function (event) {
    var form = event.target;
    if (form.matches("[data-composer]")) {
      event.preventDefault();
      submitComposer(form);
    } else if (form.matches("[data-card-edit]")) {
      event.preventDefault();
      submitEdit(form);
    } else if (form.matches("[data-task-toggle]")) {
      event.preventDefault();
      submitToggle(form);
    } else if (form.matches("[data-task-delete]")) {
      event.preventDefault();
      submitDelete(form);
    }
  });

  document.addEventListener("click", function (event) {
    var target = event.target;

    var open = target.closest("[data-edit-open]");
    if (open) {
      var card = open.closest("[data-task-card]");
      if (card) {
        event.preventDefault();
        openEdit(card);
      }
      return;
    }

    var view = target.closest("[data-card-view]");
    if (view) {
      openEdit(view.closest("[data-task-card]"));
      return;
    }

    if (target.closest("[data-edit-cancel]")) {
      closeEdit(target.closest("[data-task-card]"), { restore: true, focus: true });
      return;
    }

    var close = target.closest("[data-composer-close]");
    if (close) {
      var composer = close.closest("[data-composer]");
      resetComposer(composer);
      collapseComposer(composer);
      focusComposerTitle(composer);
    }
  });

  document.addEventListener("focusin", function (event) {
    var composer = event.target.closest && event.target.closest("[data-composer]");
    if (composer && event.target.name === "title" && !composer.dataset.keepCollapsed) {
      expandComposer(composer);
    }
  });

  // Clic fuera de la tarjeta en edición o del composer. Se escucha "click" en
  // fase de captura (no "pointerdown") para que el cierre, que cambia la altura
  // de los elementos, no desplace la página entre pulsar y soltar el botón.
  document.addEventListener("click", function (event) {
    if (editing && !editing.contains(event.target) && editing.getAttribute("aria-busy") !== "true") {
      if (event.target.closest("[data-toast-region]")) return;
      if (isDirty(editing)) {
        submitForm(editing.querySelector("[data-card-edit]"));
      } else {
        closeEdit(editing, { restore: false, focus: false });
      }
    }

    var composer = document.querySelector("[data-composer]");
    if (composer && !composer.contains(event.target) && composer.elements.title.value.trim() === "") {
      collapseComposer(composer);
    }
  }, true);

  document.addEventListener("change", function (event) {
    var input = event.target;
    if (input.name !== "color") return;
    var container = input.closest("[data-task-card]") || input.closest("[data-composer]");
    if (container) setColorClass(container, input.value);
  });

  document.addEventListener("input", function (event) {
    if (event.target.tagName === "TEXTAREA" && event.target.closest("[data-card-edit], [data-composer]")) {
      autosize(event.target);
    }
  });

  document.addEventListener("keydown", function (event) {
    var target = event.target;
    var card = target.closest && target.closest("[data-task-card]");

    if (card && card === editing) {
      if (event.key === "Escape") {
        event.preventDefault();
        closeEdit(card, { restore: true, focus: true });
        return;
      }
      if (event.key === "Enter" && (event.ctrlKey || event.metaKey)) {
        event.preventDefault();
        submitForm(card.querySelector("[data-card-edit]"));
        return;
      }
    }

    var composer = target.closest && target.closest("[data-composer]");
    if (composer && event.key === "Escape") {
      collapseComposer(composer);
      focusComposerTitle(composer);
      return;
    }

    if ((event.key === "z" || event.key === "Z") && (event.ctrlKey || event.metaKey) && !event.shiftKey) {
      if (toast && toast.undo && !isTextField(document.activeElement)) {
        event.preventDefault();
        undo();
      }
    }
  });

  initComposer(document.querySelector("[data-composer]"));
})();
