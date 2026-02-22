<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;

/**
 * WordPress bridge - the most complex bridge due to WordPress's heavy use of globals.
 *
 * Bootstraps WordPress once, then resets ALL global state between requests.
 * This is what makes maboo viable for WordPress in worker mode - something
 * FrankenPHP struggles with due to state pollution.
 */
class WordPress implements BridgeInterface
{
    public function __construct(
        private readonly string $basePath,
    ) {}

    public function bootstrap(): void
    {
        // Define WordPress constants before loading
        if (!defined('ABSPATH')) {
            define('ABSPATH', rtrim($this->basePath, '/') . '/');
        }

        // Prevent WordPress from sending headers directly
        if (!defined('WP_USE_THEMES')) {
            define('WP_USE_THEMES', true);
        }

        // Load WordPress
        require_once ABSPATH . 'wp-load.php';
    }

    public function handle(Request $request, Response $response): void
    {
        // Set up WordPress globals from the request
        $this->setupRequest($request);

        // Capture output
        ob_start();

        try {
            // Re-run WordPress main query
            wp();

            // Load the template
            if (defined('WP_USE_THEMES') && WP_USE_THEMES) {
                require ABSPATH . WPINC . '/template-loader.php';
            }

            $output = ob_get_clean();
        } catch (\Throwable $e) {
            ob_end_clean();
            $response->status(500)
                ->body('WordPress Error: ' . $e->getMessage())
                ->send();
            return;
        }

        // Get HTTP response code (WordPress might have set it)
        $statusCode = http_response_code() ?: 200;

        // Get headers that WordPress set
        $headers = [];
        foreach (headers_list() as $header) {
            $parts = explode(':', $header, 2);
            if (count($parts) === 2) {
                $headers[trim($parts[0])] = trim($parts[1]);
            }
        }

        $response->status($statusCode);
        foreach ($headers as $name => $value) {
            $response->header($name, $value);
        }
        $response->body($output ?: '');
        $response->send();
    }

    public function cleanup(): void
    {
        // === CRITICAL: Reset ALL WordPress globals ===

        global $wp, $wp_query, $wp_the_query, $post, $wp_rewrite;
        global $wp_object_cache, $wpdb;
        global $wp_actions, $wp_current_filter, $wp_filter;

        // 1. Reset main query objects
        if (isset($wp_query)) {
            $wp_query->init();
        }
        if (isset($wp_the_query)) {
            $wp_the_query->init();
        }
        if (isset($wp)) {
            $wp->init();
        }

        // 2. Reset post global
        $post = null;

        // 3. Reset rewrite rules (if modified by plugins)
        if (isset($wp_rewrite)) {
            $wp_rewrite->init();
        }

        // 4. Flush object cache (in-memory)
        if (isset($wp_object_cache) && is_object($wp_object_cache)) {
            if (method_exists($wp_object_cache, 'flush')) {
                $wp_object_cache->flush();
            }
        } else {
            wp_cache_flush();
        }

        // 5. Close and reconnect database (prevent "gone away")
        if (isset($wpdb)) {
            $wpdb->close();
            $wpdb->check_connection();
        }

        // 6. Reset superglobals
        $_GET = [];
        $_POST = [];
        $_REQUEST = [];
        $_COOKIE = [];
        $_FILES = [];

        // 7. Clear any output buffering
        while (ob_get_level() > 0) {
            ob_end_clean();
        }

        // 8. Reset HTTP response code
        http_response_code(200);

        // 9. Clear sent headers
        if (function_exists('header_remove')) {
            header_remove();
        }

        // 10. Force garbage collection
        gc_collect_cycles();
    }

    public function terminate(): void
    {
        // WordPress doesn't have a clean shutdown mechanism
        // Close database connection
        global $wpdb;
        if (isset($wpdb)) {
            $wpdb->close();
        }
    }

    private function setupRequest(Request $request): void
    {
        // Set PHP superglobals from maboo request
        $_SERVER = $request->toServerVars();
        $_GET = $request->query();
        $_POST = [];
        $_REQUEST = [];
        $_COOKIE = [];

        if (
            $request->method === 'POST' &&
            str_contains($request->header('content-type'), 'application/x-www-form-urlencoded')
        ) {
            parse_str($request->body, $_POST);
        }

        $_REQUEST = array_merge($_GET, $_POST);

        // Parse cookies
        $cookieHeader = $request->header('cookie');
        if ($cookieHeader !== '') {
            foreach (explode(';', $cookieHeader) as $cookie) {
                $parts = explode('=', trim($cookie), 2);
                if (count($parts) === 2) {
                    $_COOKIE[trim($parts[0])] = urldecode(trim($parts[1]));
                }
            }
        }

        // Set up WordPress-specific server vars
        $_SERVER['REQUEST_URI'] = $request->uri .
            ($request->queryString ? '?' . $request->queryString : '');
        $_SERVER['PHP_SELF'] = $request->uri;
        $_SERVER['SCRIPT_FILENAME'] = ABSPATH . 'index.php';
        $_SERVER['DOCUMENT_ROOT'] = ABSPATH;
    }
}
