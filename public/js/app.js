/**
 * Maboo Test Page JavaScript
 */

document.addEventListener('DOMContentLoaded', () => {
    // Counter
    let count = 0;
    const counterEl = document.getElementById('counter');
    const counterBtn = document.getElementById('counter-btn');

    counterBtn.addEventListener('click', () => {
        count++;
        counterEl.textContent = count;

        // Add animation
        counterEl.style.transform = 'scale(1.3)';
        setTimeout(() => {
            counterEl.style.transform = 'scale(1)';
        }, 150);
    });

    // AJAX Test
    const ajaxBtn = document.getElementById('ajax-btn');
    const ajaxResult = document.getElementById('ajax-result');

    ajaxBtn.addEventListener('click', async () => {
        ajaxBtn.disabled = true;
        ajaxBtn.textContent = 'Loading...';

        try {
            const response = await fetch('?ajax=1');
            const data = await response.json();

            ajaxResult.innerHTML = `<span style="color: #10b981;">✓ AJAX Request Successful!</span>\n\n` +
                JSON.stringify(data, null, 2);
        } catch (error) {
            ajaxResult.innerHTML = `<span style="color: #ef4444;">✗ Error:</span> ${error.message}`;
        } finally {
            ajaxBtn.disabled = false;
            ajaxBtn.textContent = 'Test AJAX Request';
        }
    });

    // Dynamic time update
    const updateTime = () => {
        const now = new Date();
        const timeStr = now.toLocaleTimeString();
        // Could update a clock element if we had one
    };

    setInterval(updateTime, 1000);

    // Console log for debugging
    console.log('🚀 Maboo Test Page loaded');
    console.log('PHP Version:', document.querySelector('.status-item .value.success')?.textContent);
});
