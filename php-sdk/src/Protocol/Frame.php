<?php

declare(strict_types=1);

namespace Maboo\Protocol;

class Frame
{
    public function __construct(
        public readonly int $type,
        public readonly int $flags,
        public readonly int $streamId,
        public readonly string $headers,
        public readonly string $payload,
    ) {}

    /**
     * Decode msgpack headers into array.
     */
    public function decodeHeaders(): array
    {
        if (empty($this->headers)) {
            return [];
        }
        return Msgpack::decode($this->headers);
    }
}
