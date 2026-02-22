<?php
// Maboo Test Page
header('Content-Type: text/html; charset=utf-8');

// Simulate some PHP processing
$start_time = microtime(true);
$requests_count = isset($_COOKIE['maboo_requests']) ? (int)$_COOKIE['maboo_requests'] + 1 : 1;

// Set a cookie
setcookie('maboo_requests', (string)$requests_count, time() + 3600, '/');

// Handle AJAX requests
if (isset($_GET['ajax'])) {
    header('Content-Type: application/json');
    echo json_encode([
        'status' => 'ok',
        'php_version' => PHP_VERSION,
        'timestamp' => time(),
        'memory' => round(memory_get_usage(true) / 1024 / 1024, 2) . ' MB',
        'requests' => $requests_count
    ]);
    exit;
}

// Handle POST requests
$post_result = null;
if ($_SERVER['REQUEST_METHOD'] === 'POST' && isset($_POST['name'])) {
    $post_result = [
        'message' => 'Hello, ' . htmlspecialchars($_POST['name']) . '!',
        'processed_at' => date('H:i:s')
    ];
}
?>
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Maboo Test Page</title>
    <link rel="stylesheet" href="css/style.css">
</head>
<body>
    <div class="container">
        <header>
            <h1>👻 Maboo</h1>
            <p class="tagline">Embedded PHP Server</p>
        </header>

        <main>
            <section class="card status-card">
                <h2>Server Status</h2>
                <div class="status-grid">
                    <div class="status-item">
                        <span class="label">PHP Version</span>
                        <span class="value success"><?php echo PHP_VERSION; ?></span>
                    </div>
                    <div class="status-item">
                        <span class="label">PID</span>
                        <span class="value"><?php echo getmypid(); ?></span>
                    </div>
                    <div class="status-item">
                        <span class="label">Memory</span>
                        <span class="value"><?php echo round(memory_get_usage(true) / 1024 / 1024, 2); ?> MB</span>
                    </div>
                    <div class="status-item">
                        <span class="label">Render Time</span>
                        <span class="value" id="render-time"><?php echo round((microtime(true) - $start_time) * 1000, 2); ?> ms</span>
                    </div>
                </div>
            </section>

            <section class="card">
                <h2>Interactive JavaScript Test</h2>
                <div class="test-controls">
                    <button id="ajax-btn" class="btn btn-primary">Test AJAX Request</button>
                    <button id="counter-btn" class="btn btn-secondary">Increment Counter</button>
                    <span class="counter-display">Count: <span id="counter">0</span></span>
                </div>
                <div id="ajax-result" class="result-box"></div>
            </section>

            <section class="card">
                <h2>PHP Form Test</h2>
                <?php if ($post_result): ?>
                    <div class="alert alert-success">
                        <?php echo $post_result['message']; ?> (processed at <?php echo $post_result['processed_at']; ?>)
                    </div>
                <?php endif; ?>
                <form method="POST" class="test-form">
                    <input type="text" name="name" placeholder="Enter your name" required>
                    <button type="submit" class="btn btn-primary">Submit Form</button>
                </form>
            </section>

            <section class="card">
                <h2>Session Info</h2>
                <div class="info-grid">
                    <div class="info-item">
                        <strong>Request Method:</strong> <?php echo $_SERVER['REQUEST_METHOD']; ?>
                    </div>
                    <div class="info-item">
                        <strong>Request URI:</strong> <?php echo $_SERVER['REQUEST_URI']; ?>
                    </div>
                    <div class="info-item">
                        <strong>Server Name:</strong> <?php echo $_SERVER['SERVER_NAME'] ?? 'localhost'; ?>
                    </div>
                    <div class="info-item">
                        <strong>Remote Address:</strong> <?php echo $_SERVER['REMOTE_ADDR'] ?? '127.0.0.1'; ?>
                    </div>
                    <div class="info-item">
                        <strong>User Agent:</strong> <?php echo $_SERVER['HTTP_USER_AGENT'] ?? 'Unknown'; ?>
                    </div>
                    <div class="info-item">
                        <strong>Request Count:</strong> <?php echo $requests_count; ?>
                    </div>
                </div>
            </section>

            <section class="card">
                <h2>Query Parameters</h2>
                <pre class="code-block"><?php echo empty($_GET) ? 'No query parameters' : print_r($_GET, true); ?></pre>
            </section>
        </main>

        <footer>
            <p>Powered by Maboo • PHP <?php echo PHP_VERSION; ?> • <?php echo date('Y-m-d H:i:s'); ?></p>
        </footer>
    </div>

    <script src="js/app.js"></script>
</body>
</html>
