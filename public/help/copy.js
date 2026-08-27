(function () {
    'use strict';

    var clipboard = navigator.clipboard;
    if (!clipboard || typeof clipboard.writeText !== 'function') {
        return;
    }

    if (document.documentElement.hasAttribute('data-tf-copy')) {
        return;
    }
    document.documentElement.setAttribute('data-tf-copy', '');

    var IDLE_LABEL = 'Copy';
    var COPIED_LABEL = 'Copied';
    var FAILED_LABEL = 'Press Ctrl or Cmd C';
    var FAILED_SPOKEN = 'Could not copy. Press Control or Command C.';
    var RESTORE_AFTER_MS = 1500;
    var ANNOUNCE_AFTER_MS = 50;
    var COPIED_CLASS = 'just-copied';
    var FAILED_CLASS = 'copy-failed';

    var announcer = document.createElement('div');
    announcer.className = 'visually-hidden';
    announcer.setAttribute('role', 'status');
    document.body.appendChild(announcer);

    var announceTimer = null;

    function announce(message) {
        window.clearTimeout(announceTimer);
        announcer.textContent = '';
        announceTimer = window.setTimeout(function () {
            announcer.textContent = message;
        }, ANNOUNCE_AFTER_MS);
    }

    function sourceText(element) {
        var code = element.querySelector('code');
        return (code || element).textContent;
    }

    function copy(element, spoken) {
        var written;
        try {
            written = clipboard.writeText(sourceText(element));
        } catch (refused) {
            written = Promise.reject(refused);
        }

        return written.then(function () {
            if (spoken) {
                announce(COPIED_LABEL);
            }
            return true;
        }, function () {
            if (spoken) {
                announce(FAILED_SPOKEN);
            }
            return false;
        });
    }

    function restoring(element, restore) {
        return function () {
            window.clearTimeout(element.tfCopyTimer);
            element.tfCopyTimer = window.setTimeout(restore, RESTORE_AFTER_MS);
        };
    }

    function frameFor(block) {
        var frame = document.createElement('div');
        frame.className = 'example';
        block.parentNode.insertBefore(frame, block);
        frame.appendChild(block);
        return frame;
    }

    function headingFor(frame) {
        var node = frame.previousElementSibling;
        while (node !== null) {
            if (/^H[1-6]$/.test(node.tagName)) {
                return node.textContent;
            }
            node = node.previousElementSibling;
        }
        return '';
    }

    function addButton(block) {
        var frame = frameFor(block);
        var heading = headingFor(frame);

        var bar = document.createElement('div');
        bar.className = 'copy-bar';

        var button = document.createElement('button');
        button.type = 'button';
        button.className = 'copy-button';
        button.textContent = IDLE_LABEL;
        if (heading !== '') {
            button.setAttribute('aria-label', IDLE_LABEL + ': ' + heading);
        }

        var schedule = restoring(button, function () {
            button.textContent = IDLE_LABEL;
        });

        button.addEventListener('click', function () {
            copy(block, false).then(function (copied) {
                button.textContent = copied ? COPIED_LABEL : FAILED_LABEL;
                schedule();
            }).catch(function () {
                button.textContent = FAILED_LABEL;
                schedule();
            });
        });

        bar.appendChild(button);
        frame.insertBefore(bar, block);
        block.classList.add('has-copy-button');
    }

    function addInlineCopy(token) {
        var schedule = restoring(token, function () {
            token.classList.remove(COPIED_CLASS);
            token.classList.remove(FAILED_CLASS);
        });

        function mark(copied) {
            token.classList.remove(COPIED_CLASS);
            token.classList.remove(FAILED_CLASS);
            token.classList.add(copied ? COPIED_CLASS : FAILED_CLASS);
            schedule();
        }

        token.addEventListener('click', function () {
            copy(token, true).then(mark).catch(function () {
                mark(false);
            });
        });
    }

    Array.prototype.forEach.call(document.querySelectorAll('pre.copyable'), addButton);
    Array.prototype.forEach.call(document.querySelectorAll('code.copyable'), addInlineCopy);
}());
