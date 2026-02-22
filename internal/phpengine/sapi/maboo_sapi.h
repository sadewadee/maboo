#ifndef MABOO_SAPI_H
#define MABOO_SAPI_H

#include <stddef.h>

// Opaque PHP context handle
typedef struct php_context php_context;

// Create a new PHP context
php_context* php_context_new(void);

// Set superglobal values
void php_context_set_server(php_context* ctx, const char* key, const char* value);
void php_context_set_get(php_context* ctx, const char* key, const char* value);
void php_context_set_post(php_context* ctx, const char* key, const char* value);
void php_context_set_cookie(php_context* ctx, const char* key, const char* value);
void php_context_set_env(php_context* ctx, const char* key, const char* value);

// Set document root and script
void php_context_set_document_root(php_context* ctx, const char* root);
void php_context_set_script_filename(php_context* ctx, const char* filename);

// Free context
void php_context_free(php_context* ctx);

// PHP engine lifecycle
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
