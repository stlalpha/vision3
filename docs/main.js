/* ViSiON/3 BBS - Minimal site effects */

(function () {
    'use strict';

    /* Fade-in sections on scroll */
    var sections = document.querySelectorAll('section, .hero');

    if (!('IntersectionObserver' in window)) {
        sections.forEach(function (section) {
            section.classList.add('visible');
        });
        return;
    }

    var observer = new IntersectionObserver(function (entries) {
        entries.forEach(function (entry) {
            if (entry.isIntersecting) {
                entry.target.classList.add('visible');
            }
        });
    }, { threshold: 0.1 });

    sections.forEach(function (section) {
        section.classList.add('fade-in');
        observer.observe(section);
    });
})();

/* ---- Telix Dialer Splash Screen ---- */

// Cookie helpers
function hasVisitedBefore() {
    return document.cookie.includes('vision3_visited=1');
}

function setVisitedCookie() {
    var expires = new Date(Date.now() + 30 * 24 * 60 * 60 * 1000).toUTCString();
    document.cookie = 'vision3_visited=1; expires=' + expires + '; path=/; SameSite=Lax';
}

// DTMF frequency pairs (ITU-T Q.23)
var DTMF_FREQUENCIES = {
    '1': [697, 1209], '2': [697, 1336], '3': [697, 1477],
    '4': [770, 1209], '5': [770, 1336], '6': [770, 1477],
    '7': [852, 1209], '8': [852, 1336], '9': [852, 1477],
    '*': [941, 1209], '0': [941, 1336], '#': [941, 1477]
};

function playTone(audioContext, frequencies, duration, startTime) {
    frequencies.forEach(function (freq) {
        var oscillator = audioContext.createOscillator();
        var gainNode = audioContext.createGain();
        oscillator.type = 'sine';
        oscillator.frequency.value = freq;
        gainNode.gain.value = 0.15;
        oscillator.connect(gainNode);
        gainNode.connect(audioContext.destination);
        oscillator.start(startTime);
        oscillator.stop(startTime + duration);
    });
}

function playDialTone(audioContext, startTime, duration) {
    playTone(audioContext, [350, 440], duration, startTime);
}

function playRingTone(audioContext, startTime) {
    // US ring: 440+480 Hz, 2s on, 4s off
    playTone(audioContext, [440, 480], 2.0, startTime);
}

function playPhonePickup() {
    // Play phone pickup click sound
    var click = new Audio('audio/phone-pickup.mp3');
    click.volume = 0.6;
    click.play().catch(function (err) {
        console.error('Phone pickup click failed:', err);
    });
}

// Typewriter text output
function typeText(terminal, text, callback) {
    var index = 0;
    var cursor = terminal.querySelector('.telix-cursor');
    function typeChar() {
        if (index < text.length) {
            cursor.insertAdjacentText('beforebegin', text[index]);
            index++;
            setTimeout(typeChar, 30 + Math.random() * 20);
        } else if (callback) {
            callback();
        }
    }
    typeChar();
}

function printLine(terminal, text) {
    var cursor = terminal.querySelector('.telix-cursor');
    cursor.insertAdjacentText('beforebegin', text + '\n');
}

// Build Telix status bar with flexbox layout
function buildStatusBar(isOnline) {
    var statusBar = document.querySelector('.telix-status-bar');
    if (!statusBar) return;

    var leftContent = 'Unregistered';
    var middleContent = '| ANSI-BBS | 38400-N81 FAX | | | |';
    var rightContent = isOnline ? 'Online 00:00' : 'Offline';

    statusBar.innerHTML =
        '<span>' + leftContent + '</span>' +
        '<span>' + middleContent + '</span>' +
        '<span>' + rightContent + '</span>';
}

// Main dialer sequence - chained callbacks for synchronous execution
function runDialerSequence(splash) {
    var terminal = document.getElementById('telix-terminal');
    var audioContext = new (window.AudioContext || window.webkitAudioContext)();
    var phoneDigits = '13145673833';

    // Clear the "Click to connect..." prompt
    var prompt = terminal.querySelector('.telix-prompt');
    if (prompt) prompt.remove();

    // Preload modem screech
    var modemAudio = new Audio('audio/modem-handshake.mp3');
    modemAudio.volume = 0.8;

    // Phase 1: Type AT&F, wait for OK
    typeText(terminal, 'AT&F\n', function () {
        printLine(terminal, 'OK');

        // Phase 2: Wait, then type init string
        setTimeout(function () {
            typeText(terminal, 'AT&C1&D2&K3&M4&N6\n', function () {
                printLine(terminal, 'OK');

                // Phase 3: Wait, then type ATDT
                setTimeout(function () {
                    var atdtChars = 'ATDT';
                    var atdtIndex = 0;
                    var cursor = terminal.querySelector('.telix-cursor');

                    var atdtInterval = setInterval(function () {
                        if (atdtIndex < atdtChars.length) {
                            cursor.insertAdjacentText('beforebegin', atdtChars[atdtIndex]);
                            atdtIndex++;
                        } else {
                            clearInterval(atdtInterval);

                            // Click sound immediately after ATDT typed
                            playPhonePickup();

                            // Wait 0.5s, then start dial tone
                            setTimeout(function () {
                                playDialTone(audioContext, audioContext.currentTime, 1.5);

                                // Phase 4: Type phone number with DTMF
                                setTimeout(function () {
                                    var digitIndex = 0;
                                    var digitInterval = setInterval(function () {
                                        if (digitIndex < phoneDigits.length) {
                                            var digit = phoneDigits[digitIndex];
                                            cursor.insertAdjacentText('beforebegin', digit);

                                            // Play DTMF for this digit
                                            var freqs = DTMF_FREQUENCIES[digit];
                                            if (freqs) {
                                                playTone(audioContext, freqs, 0.1, audioContext.currentTime);
                                            }
                                            digitIndex++;
                                        } else {
                                            clearInterval(digitInterval);
                                            cursor.insertAdjacentText('beforebegin', '\n');

                                            // Phase 5: First ring
                                            setTimeout(function () {
                                                playRingTone(audioContext, audioContext.currentTime);

                                                // Phase 6: Wait 4.5s, then second ring
                                                setTimeout(function () {
                                                    playRingTone(audioContext, audioContext.currentTime);

                                                    // Show RING text shortly after second ring starts
                                                    setTimeout(function () {
                                                        printLine(terminal, 'RING');

                                                    // Phase 7: Wait 1.5s, phone pickup click
                                                    setTimeout(function () {
                                                        console.log('Phone pickup click...');
                                                        playPhonePickup();

                                                        // Phase 8: Wait 0.5s, then modem screech (plays once, ~5s)
                                                        setTimeout(function () {
                                                            console.log('Starting modem handshake audio...');

                                                            // Play screech file once
                                                            modemAudio.play().then(function() {
                                                                console.log('Modem audio playing successfully');
                                                            }).catch(function (err) {
                                                                console.error('Modem audio playback failed:', err);
                                                            });

                                                            // Phase 9: CONNECT after screech finishes (~18.5s)
                                                            setTimeout(function () {
                                                                printLine(terminal, 'CONNECT 14400/ARQ/V32bis/LAPM');

                                                                // Update status bar to ONLINE
                                                                buildStatusBar(true);

                                                                // Phase 10: Remove overlay after brief pause
                                                                setTimeout(function () {
                                                                    audioContext.close();
                                                                    splash.remove();
                                                                    document.body.classList.remove('splash-active');
                                                                    setVisitedCookie();
                                                                }, 2000);
                                                            }, 18500);
                                                        }, 500);
                                                    }, 1500);
                                                    }, 200);
                                                }, 4500);
                                        }, 1000);
                                    }
                                    }, 180);
                                }, 200);
                            }, 500);
                        }
                    }, 50);
                }, 300);
            });
        }, 300);
    });
}

// Initialize
(function () {
    var splash = document.getElementById('telix-splash');
    if (!splash) return;

    // Skip splash if user prefers reduced motion
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
        splash.remove();
        return;
    }

    if (hasVisitedBefore()) {
        splash.remove();
        return;
    }

    document.body.classList.add('splash-active');

    // Build status bar after page is fully rendered
    setTimeout(function () {
        buildStatusBar(false);
    }, 100);

    function startSequence() {
        splash.removeEventListener('click', clickHandler);
        splash.removeEventListener('keydown', keyHandler);
        splash.style.cursor = 'default';
        runDialerSequence(splash);
    }

    function clickHandler() {
        startSequence();
    }

    function keyHandler(e) {
        if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            startSequence();
        }
    }

    splash.addEventListener('click', clickHandler);
    splash.addEventListener('keydown', keyHandler);
})();

/* ---- BBS Menu Keyboard Navigation ---- */
(function () {
    'use strict';

    var keymap = {
        'a': '#about',
        'A': '#about',
        'f': '#features',
        'F': '#features',
        'g': '#get-started',
        'G': '#get-started',
        'j': '#get-involved',
        'J': '#get-involved',
        'h': '#history',
        'H': '#history'
    };

    document.addEventListener('keydown', function (e) {
        // Don't hijack if modifier keys are pressed or user is typing
        if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) {
            return;
        }

        if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' ||
            e.target.isContentEditable || e.target.matches('[contenteditable="true"]')) {
            return;
        }

        var section = keymap[e.key];
        if (section) {
            e.preventDefault();
            var element = document.querySelector(section);
            if (element) {
                element.scrollIntoView({ behavior: 'smooth' });
            }
        }
    });
})();
