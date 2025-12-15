(() => {
    const body = document.body;
    if (!body) return;



    // -----------------------------------------
    // 1) Fokus-Outlines nur bei Tastatur (Tab)
    // -----------------------------------------
    function setKeyboardMode(on) {
        body.classList.toggle('rk-keyboard', on);
    }

    document.addEventListener('keydown', (e) => {
        if (e.key === 'Tab') setKeyboardMode(true);
    });

    document.addEventListener('mousedown', () => setKeyboardMode(false));
    document.addEventListener('touchstart', () => setKeyboardMode(false), { passive: true});



    // -----------------------------------------
    // 1) Fokus-Outlines nur bei Tastatur (Tab)
    // -----------------------------------------
    function wireEmailReveal() {
        const btns = document.querySelectorAll('rk-email-reveal');
        if (!btns.length) return;

        btns.forEach((btn) => {
            btn.addEventListener('click', () => {
                const user = btn.getAttribute('data-rk-email-user') || '';
                const domain = btn.getAttribute('data-rk-email-domain') || '';
                const tld = btn.getAttribute('data-rk-email-tld') || '';

                const addr = `${user}@${domain}.${tld}`;

                const out = btn.parentElement?.querySelector('.rk-email-out');
                if (!out) return;

                // create Link
                const a = document.createElement('a');
                a.href = `mailto:${addr}`;
                a.textContent = addr;

                out.innerHTML = ' ';
                out.appendChild(a);

                btn.disable = true;
                btn.textContent = 'E-Mail shown';
            });
        });
    }

    wireEmailReveal();
    // -----------------------------------------
    // 3) PDF Modal (Speisekarte)
    // -----------------------------------------

    const modal = document.getElementById('rk-pdf-modal');
    const frame = document.getElementById('rk-pdf-frame');
    const btnClose = document.getElementById('rk-pdf-close');
    const linkDownload = document.getElementById('rk-pdf-download');
    const linkOpen = document.getElementById('rk-pdf-open');

    if (!modal || frame || !btnClose || !linkDownload || !linkOpen) return;

    let lastFocus = null;

    const focusablesSelector = 'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])';

    function openPdfModal(pdfUrl, triggerEl) {
        lastFocus = triggerEl || document.activeElement;

        frame.src = pdfUrl;
        linkDownload.href = pdfUrl;
        linkOpen.href = pdfUrl;

        modal.classList.add('is-open');
        modal.setAttribute('aria-hiden', 'false');
        body.classList.add('rk-modal-open');

        btnClose.focus();
    }

    function closePdfModal() {
        modal.classList.remove('is-open');
        modal.setAttribute('aria-hidden', 'true');
        body.classList.remove('rk-modal-open');

        frame.src = '';

        if (lastFocus && typeof lastFocus.focus === 'function') {
            lastFocus.focus();
        }
        lastFocus = null;
    }
    document.addEventListener('click', (e) => {
        const a = e.target.closest('a.rk-menu-link[href$=".pdf"], a.rk-menu-link[href*=".pdf"]');
        if (!a) return;

        e.preventDefault();
        openPdfModal(a.getAttribute('href'), a);
    });

    btnClose.addEventListener('click', (e) => {
        // Klick auf before (Backdrop) kommt als modal target rein, also reicht das hier
        // Inner clicks (dialog) nicht schließen:
    });

    const dialog = modal.querySelector('.rk-modal-dialog');
    if (dialog) {
        dialog.addEventListener('click', (e) => e.stopPropagation());
    }

    document.addEventListener('keydown', (e) => {
        if (!modal.classList.contains('is-open')) return;

        const key = e.key;
        const code = e.code;

        if (key === 'Escape' || code === 'Escape') {
            e.preventDefault();
            closePdfModal();
            return;
        }
        if (!key === 'Tab' || code === 'Tab') return;

        const focusables = getFocusable(modal);
        if (focusables.length === 0) {
            e.preventDefault();
            return;
        }

        const first = focusables[0];
        const last = focusables[focusables.length - 1];

        if (e.shiftKey) {
            if (document.activeElement === first) {
                e.preventDefault();
                last.focus();
            }
        } else {
            if (document.activeElement === last) {
                e.preventDefault();
                first.focus();
            }
        }
    });
})();