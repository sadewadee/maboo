#ifndef MABOO_SAPI_H
#define MABOO_SAPI_H

#include <stddef.h>

// Build-time toggle: when compiling with available embed headers/symbols,
// define PHP_EMBED_AVAILABLE via CFLAGS.

// Opaque PHP context handle
typedef struct php_context php_context;

// Context management
php_context* php_context_new(void);
void php_context_free(php_context* ctx);

// Set thread index for Go callback routing
void php_context_set_thread_index(php_context* ctx, int index);

// Set superglobal values
void php_context_set_server(php_context* ctx, const char* key, const char* value);
void php_context_set_get(php_context* ctx, const char* key, const char* value);
void php_context_set_post(php_context* ctx, const char* key, const char* value);
void php_context_set_cookie(php_context* ctx, const char* key, const char* value);
void php_context_set_env(php_context* ctx, const char* key, const char* value);

// Set document root and script
void php_context_set_document_root(php_context* ctx, const char* root);
void php_context_set_script_filename(php_context* ctx, const char* filename);

// Set POST data (raw body)
void php_context_set_post_data(php_context* ctx, const char* data, size_t len);

// PHP engine lifecycle (staged, compile-safe interface)
// Startup/Shutdown currently prepare scaffolding. Full Zend/SAPI lifecycle
// integration will be implemented in a follow-up embedding phase.
int php_engine_startup(const char* version);
void php_engine_shutdown(void);

// Execute a PHP script
typedef struct {
    int status;
    char* headers;
    size_t headers_len;
    char* body;
    size_t body_len;
} php_response;

php_response* php_execute(php_context* ctx, const char* script);
void php_response_free(php_response* resp);

#endif // MABOO_SAPI_H
