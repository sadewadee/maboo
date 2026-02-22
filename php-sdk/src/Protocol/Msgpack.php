<?php

declare(strict_types=1);

namespace Maboo\Protocol;

/**
 * Minimal msgpack encoder/decoder for maboo protocol headers.
 * Pure PHP implementation - no extension required.
 * Supports: strings, integers, booleans, null, arrays, maps.
 */
class Msgpack
{
    public static function encode(mixed $value): string
    {
        if (is_null($value)) {
            return "\xc0";
        }
        if (is_bool($value)) {
            return $value ? "\xc3" : "\xc2";
        }
        if (is_int($value)) {
            return self::encodeInt($value);
        }
        if (is_float($value)) {
            return "\xcb" . pack('E', $value); // float64
        }
        if (is_string($value)) {
            return self::encodeString($value);
        }
        if (is_array($value)) {
            if (array_is_list($value)) {
                return self::encodeArray($value);
            }
            return self::encodeMap($value);
        }
        throw new \InvalidArgumentException('Unsupported type: ' . gettype($value));
    }

    public static function decode(string $data): mixed
    {
        $offset = 0;
        return self::decodeValue($data, $offset);
    }

    private static function encodeInt(int $value): string
    {
        if ($value >= 0 && $value <= 0x7f) {
            return chr($value);
        }
        if ($value < 0 && $value >= -32) {
            return chr($value & 0xff);
        }
        if ($value >= 0 && $value <= 0xff) {
            return "\xcc" . chr($value);
        }
        if ($value >= 0 && $value <= 0xffff) {
            return "\xcd" . pack('n', $value);
        }
        if ($value >= 0 && $value <= 0xffffffff) {
            return "\xce" . pack('N', $value);
        }
        if ($value < 0 && $value >= -128) {
            return "\xd0" . pack('c', $value);
        }
        if ($value < 0 && $value >= -32768) {
            return "\xd1" . pack('n', $value & 0xffff);
        }
        if ($value < 0 && $value >= -2147483648) {
            return "\xd2" . pack('N', $value & 0xffffffff);
        }
        return "\xd3" . pack('J', $value);
    }

    private static function encodeString(string $value): string
    {
        $len = strlen($value);
        if ($len <= 31) {
            return chr(0xa0 | $len) . $value;
        }
        if ($len <= 0xff) {
            return "\xd9" . chr($len) . $value;
        }
        if ($len <= 0xffff) {
            return "\xda" . pack('n', $len) . $value;
        }
        return "\xdb" . pack('N', $len) . $value;
    }

    private static function encodeArray(array $value): string
    {
        $count = count($value);
        if ($count <= 15) {
            $result = chr(0x90 | $count);
        } elseif ($count <= 0xffff) {
            $result = "\xdc" . pack('n', $count);
        } else {
            $result = "\xdd" . pack('N', $count);
        }
        foreach ($value as $item) {
            $result .= self::encode($item);
        }
        return $result;
    }

    private static function encodeMap(array $value): string
    {
        $count = count($value);
        if ($count <= 15) {
            $result = chr(0x80 | $count);
        } elseif ($count <= 0xffff) {
            $result = "\xde" . pack('n', $count);
        } else {
            $result = "\xdf" . pack('N', $count);
        }
        foreach ($value as $k => $v) {
            $result .= self::encode((string)$k);
            $result .= self::encode($v);
        }
        return $result;
    }

    private static function decodeValue(string $data, int &$offset): mixed
    {
        if ($offset >= strlen($data)) {
            throw new \RuntimeException('Unexpected end of msgpack data');
        }

        $byte = ord($data[$offset]);
        $offset++;

        // Positive fixint (0x00 - 0x7f)
        if ($byte <= 0x7f) {
            return $byte;
        }
        // Fixmap (0x80 - 0x8f)
        if (($byte & 0xf0) === 0x80) {
            return self::decodeMapItems($data, $offset, $byte & 0x0f);
        }
        // Fixarray (0x90 - 0x9f)
        if (($byte & 0xf0) === 0x90) {
            return self::decodeArrayItems($data, $offset, $byte & 0x0f);
        }
        // Fixstr (0xa0 - 0xbf)
        if (($byte & 0xe0) === 0xa0) {
            $len = $byte & 0x1f;
            $str = substr($data, $offset, $len);
            $offset += $len;
            return $str;
        }
        // Negative fixint (0xe0 - 0xff)
        if ($byte >= 0xe0) {
            return $byte - 256;
        }

        return match ($byte) {
            0xc0 => null,
            0xc2 => false,
            0xc3 => true,
            0xca => self::readFloat32($data, $offset),
            0xcb => self::readFloat64($data, $offset),
            0xcc => self::readUint8($data, $offset),
            0xcd => self::readUint16($data, $offset),
            0xce => self::readUint32($data, $offset),
            0xcf => self::readUint64($data, $offset),
            0xd0 => self::readInt8($data, $offset),
            0xd1 => self::readInt16($data, $offset),
            0xd2 => self::readInt32($data, $offset),
            0xd3 => self::readInt64($data, $offset),
            0xd9 => self::readStr8($data, $offset),
            0xda => self::readStr16($data, $offset),
            0xdb => self::readStr32($data, $offset),
            0xdc => self::decodeArrayItems($data, $offset, self::readRawUint16($data, $offset)),
            0xdd => self::decodeArrayItems($data, $offset, self::readRawUint32($data, $offset)),
            0xde => self::decodeMapItems($data, $offset, self::readRawUint16($data, $offset)),
            0xdf => self::decodeMapItems($data, $offset, self::readRawUint32($data, $offset)),
            default => throw new \RuntimeException(sprintf('Unsupported msgpack type: 0x%02x', $byte)),
        };
    }

    private static function readFloat32(string $data, int &$offset): float
    {
        $val = unpack('G', substr($data, $offset, 4))[1];
        $offset += 4;
        return $val;
    }

    private static function readFloat64(string $data, int &$offset): float
    {
        $val = unpack('E', substr($data, $offset, 8))[1];
        $offset += 8;
        return $val;
    }

    private static function readUint8(string $data, int &$offset): int
    {
        $val = ord($data[$offset]);
        $offset++;
        return $val;
    }

    private static function readUint16(string $data, int &$offset): int
    {
        $val = unpack('n', substr($data, $offset, 2))[1];
        $offset += 2;
        return $val;
    }

    private static function readUint32(string $data, int &$offset): int
    {
        $val = unpack('N', substr($data, $offset, 4))[1];
        $offset += 4;
        return $val;
    }

    private static function readUint64(string $data, int &$offset): int
    {
        $val = unpack('J', substr($data, $offset, 8))[1];
        $offset += 8;
        return $val;
    }

    private static function readInt8(string $data, int &$offset): int
    {
        $val = unpack('c', $data[$offset])[1];
        $offset++;
        return $val;
    }

    private static function readInt16(string $data, int &$offset): int
    {
        $val = unpack('n', substr($data, $offset, 2))[1];
        $offset += 2;
        if ($val >= 0x8000) {
            $val -= 0x10000;
        }
        return $val;
    }

    private static function readInt32(string $data, int &$offset): int
    {
        $val = unpack('N', substr($data, $offset, 4))[1];
        $offset += 4;
        if ($val >= 0x80000000) {
            $val -= 0x100000000;
        }
        return $val;
    }

    private static function readInt64(string $data, int &$offset): int
    {
        $val = unpack('J', substr($data, $offset, 8))[1];
        $offset += 8;
        return $val;
    }

    private static function readStr8(string $data, int &$offset): string
    {
        $len = ord($data[$offset]);
        $offset++;
        $str = substr($data, $offset, $len);
        $offset += $len;
        return $str;
    }

    private static function readStr16(string $data, int &$offset): string
    {
        $len = unpack('n', substr($data, $offset, 2))[1];
        $offset += 2;
        $str = substr($data, $offset, $len);
        $offset += $len;
        return $str;
    }

    private static function readStr32(string $data, int &$offset): string
    {
        $len = unpack('N', substr($data, $offset, 4))[1];
        $offset += 4;
        $str = substr($data, $offset, $len);
        $offset += $len;
        return $str;
    }

    private static function readRawUint16(string $data, int &$offset): int
    {
        $val = unpack('n', substr($data, $offset, 2))[1];
        $offset += 2;
        return $val;
    }

    private static function readRawUint32(string $data, int &$offset): int
    {
        $val = unpack('N', substr($data, $offset, 4))[1];
        $offset += 4;
        return $val;
    }

    private static function decodeArrayItems(string $data, int &$offset, int $count): array
    {
        $result = [];
        for ($i = 0; $i < $count; $i++) {
            $result[] = self::decodeValue($data, $offset);
        }
        return $result;
    }

    private static function decodeMapItems(string $data, int &$offset, int $count): array
    {
        $result = [];
        for ($i = 0; $i < $count; $i++) {
            $key = self::decodeValue($data, $offset);
            $result[(string)$key] = self::decodeValue($data, $offset);
        }
        return $result;
    }
}
