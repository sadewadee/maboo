<?php

declare(strict_types=1);

namespace Maboo\Bridge;

use Maboo\Request;
use Maboo\Response;

/**
 * Symfony framework bridge.
 *
 * Integrates with Symfony's Runtime component for proper request handling.
 * Reboots the kernel between requests to ensure clean state.
 */
class Symfony implements BridgeInterface
{
    private object $kernel; // Symfony\Component\HttpKernel\KernelInterface
    private string $environment;
    private bool $debug;

    public function __construct(
        private readonly string $basePath,
        string $environment = 'prod',
        bool $debug = false,
    ) {
        $this->environment = $environment;
        $this->debug = $debug;
    }

    public function bootstrap(): void
    {
        require $this->basePath . '/config/bootstrap.php';

        $kernelClass = $_SERVER['APP_KERNEL_CLASS'] ?? 'App\Kernel';
        $this->kernel = new $kernelClass(
            $this->environment,
            $this->debug,
        );
        $this->kernel->boot();
    }

    public function handle(Request $request, Response $response): void
    {
        // Convert to Symfony HttpFoundation Request
        $sfRequest = $this->createSymfonyRequest($request);

        // Handle through Symfony kernel
        $sfResponse = $this->kernel->handle($sfRequest);

        // Convert back to Maboo response
        $response->status($sfResponse->getStatusCode());

        foreach ($sfResponse->headers->allPreserveCaseWithoutCookies() as $name => $values) {
            $response->header($name, implode(', ', $values));
        }

        foreach ($sfResponse->headers->getCookies() as $cookie) {
            $response->header('Set-Cookie', (string) $cookie);
        }

        $response->body($sfResponse->getContent());
        $response->send();

        // Terminate kernel
        $this->kernel->terminate($sfRequest, $sfResponse);
    }

    public function cleanup(): void
    {
        if (!isset($this->kernel)) {
            return;
        }

        // Symfony handles state cleanup well via kernel reboot
        // Reboot resets the container and all services
        $this->kernel->shutdown();
        $this->kernel->boot();

        // Reset resettable services (services tagged with kernel.reset)
        $container = $this->kernel->getContainer();
        if ($container->has('services_resetter')) {
            $container->get('services_resetter')->reset();
        }
    }

    public function terminate(): void
    {
        if (isset($this->kernel)) {
            $this->kernel->shutdown();
        }
    }

    private function createSymfonyRequest(Request $request): object
    {
        return new \Symfony\Component\HttpFoundation\Request(
            $request->query(),
            $this->parsePost($request),
            [],
            $_COOKIE,
            $_FILES,
            $request->toServerVars(),
            $request->body,
        );
    }

    private function parsePost(Request $request): array
    {
        $post = [];
        if (
            $request->method === 'POST' &&
            str_contains($request->header('content-type'), 'application/x-www-form-urlencoded')
        ) {
            parse_str($request->body, $post);
        }
        return $post;
    }
}
