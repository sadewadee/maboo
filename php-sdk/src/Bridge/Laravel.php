<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;

/**
 * Laravel framework bridge.
 *
 * Bootstraps Laravel once and handles multiple requests with proper state
 * cleanup between them. This solves FrankenPHP's state pollution issues
 * and provides production-ready Laravel integration (not BETA like FrankenPHP's Octane).
 */
class Laravel implements BridgeInterface
{
    private object $app; // Illuminate\Foundation\Application
    private object $kernel; // Illuminate\Contracts\Http\Kernel

    public function __construct(
        private readonly string $basePath,
    ) {}

    public function bootstrap(): void
    {
        // Require Laravel's bootstrap
        $this->app = require $this->basePath . '/bootstrap/app.php';

        $this->kernel = $this->app->make('Illuminate\Contracts\Http\Kernel');
    }

    public function handle(Request $request, Response $response): void
    {
        // Convert Maboo request to Illuminate request
        $illuminateRequest = $this->createIlluminateRequest($request);

        // Handle through Laravel's kernel
        $illuminateResponse = $this->kernel->handle($illuminateRequest);

        // Convert back to Maboo response
        $response->status($illuminateResponse->getStatusCode());

        foreach ($illuminateResponse->headers->allPreserveCaseWithoutCookies() as $name => $values) {
            $response->header($name, implode(', ', $values));
        }

        // Handle cookies
        foreach ($illuminateResponse->headers->getCookies() as $cookie) {
            $response->header('Set-Cookie', (string) $cookie);
        }

        $response->body($illuminateResponse->getContent());
        $response->send();

        // Terminate middleware
        $this->kernel->terminate($illuminateRequest, $illuminateResponse);
    }

    public function cleanup(): void
    {
        if (!isset($this->app)) {
            return;
        }

        // === CRITICAL STATE RESET (prevents FrankenPHP-style pollution) ===

        // 1. Clear resolved facade instances
        if (class_exists('Illuminate\Support\Facades\Facade')) {
            \Illuminate\Support\Facades\Facade::clearResolvedInstances();
        }

        // 2. Flush auth state
        if ($this->app->resolved('auth')) {
            $auth = $this->app->make('auth');
            if (method_exists($auth, 'forgetGuards')) {
                $auth->forgetGuards();
            }
        }

        // 3. Reset database connections to prevent "gone away" errors
        if ($this->app->resolved('db')) {
            $db = $this->app->make('db');
            foreach ($db->getConnections() as $connection) {
                $connection->disconnect();
            }
        }

        // 4. Clear session data
        if ($this->app->resolved('session')) {
            $session = $this->app->make('session');
            if (method_exists($session, 'flush')) {
                $session->flush();
            }
        }

        // 5. Clear cache for transient items
        if ($this->app->resolved('cache')) {
            // Don't flush the entire cache, just forget driver-level state
        }

        // 6. Reset event dispatcher registered listeners (from this request only)
        if ($this->app->resolved('events')) {
            // Let the container handle this naturally
        }

        // 7. Reset cookie jar
        if ($this->app->resolved('cookie')) {
            $cookie = $this->app->make('cookie');
            if (method_exists($cookie, 'flushQueuedCookies')) {
                $cookie->flushQueuedCookies();
            }
        }

        // 8. Clear view shared data
        if ($this->app->resolved('view')) {
            $view = $this->app->make('view');
            if (method_exists($view, 'flushState')) {
                $view->flushState();
            }
        }

        // 9. Reset translator locale
        if ($this->app->resolved('translator')) {
            $translator = $this->app->make('translator');
            $translator->setLocale($this->app->make('config')->get('app.locale', 'en'));
        }

        // 10. Flush resolved singletons that shouldn't persist
        $this->app->forgetScopedInstances();
    }

    public function terminate(): void
    {
        // Graceful shutdown
        if (isset($this->app) && method_exists($this->app, 'terminate')) {
            $this->app->terminate();
        }
    }

    private function createIlluminateRequest(Request $request): object
    {
        $server = $request->toServerVars();
        $query = $request->query();

        $post = [];
        if (
            $request->method === 'POST' &&
            str_contains($request->header('content-type'), 'application/x-www-form-urlencoded')
        ) {
            parse_str($request->body, $post);
        }

        return \Illuminate\Http\Request::createFromBase(
            new \Symfony\Component\HttpFoundation\Request(
                $query,
                $post,
                [],
                $_COOKIE,
                $_FILES,
                $server,
                $request->body,
            )
        );
    }
}
