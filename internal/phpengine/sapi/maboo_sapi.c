#include "maboo_sapi.h"
#ifdef PHP_EMBED_AVAILABLE
#include <php.h>
#include <php_main.h>
#include <SAPI.h>
#endif
#include <stdlib.h>
#include <string.h>

// Thread-local storage for request context
static __thread php_context* current_context = NULL;
static __thread int current_thread_index = 0;

// Go callback exports (implemented in callbacks.go)
extern size_t go_ub_write(int thread_idx, const char* str, size_t len);
extern int go_send_headers(int thread_idx, int status, const char* headers, size_t headers_len);
extern size_t go_read_post(int thread_idx, char* buffer, size_t len);
extern char* go_read_cookies(int thread_idx);
extern void go_register_variables(int thread_idx, const char* key, const char* value);
extern void go_log_message(int thread_idx, const char* message, int type);

// Internal context structure
struct php_context {
    int thread_index;
    char* document_root;
    char* script_filename;
    char* post_data;
    size_t post_data_len;
    size_t post_data_read;

    // Server variables storage (simple key-value pairs)
    char** server_keys;
    char** server_values;
    size_t server_count;
    size_t server_capacity;

    // Output buffering
    char* output_buffer;
    size_t output_len;
    size_t output_capacity;

    // Headers
    char* headers_buffer;
    size_t headers_len;
    int http_status;
};

// Context management functions
php_context* php_context_new(void) {
    php_context* ctx = calloc(1, sizeof(php_context));
    if (!ctx) {
        return NULL;
    }

    ctx->output_capacity = 8192;
    ctx->output_buffer = malloc(ctx->output_capacity);
    if (!ctx->output_buffer) {
        free(ctx);
        return NULL;
    }

    ctx->server_capacity = 64;
    ctx->server_keys = calloc(ctx->server_capacity, sizeof(char*));
    ctx->server_values = calloc(ctx->server_capacity, sizeof(char*));
    if (!ctx->server_keys || !ctx->server_values) {
        free(ctx->output_buffer);
        free(ctx->server_keys);
        free(ctx->server_values);
        free(ctx);
        return NULL;
    }

    ctx->http_status = 200;
    return ctx;
}

void php_context_set_thread_index(php_context* ctx, int index) {
    if (ctx) {
        ctx->thread_index = index;
    }
}

static void php_context_add_server_var(php_context* ctx, const char* key, const char* value) {
    if (!ctx || !key) return;

    if (ctx->server_count >= ctx->server_capacity) {
        size_t new_capacity = ctx->server_capacity * 2;
        char** new_keys = realloc(ctx->server_keys, new_capacity * sizeof(char*));
        char** new_values = realloc(ctx->server_values, new_capacity * sizeof(char*));
        if (!new_keys || !new_values) return;
        ctx->server_keys = new_keys;
        ctx->server_values = new_values;
        ctx->server_capacity = new_capacity;
    }

    ctx->server_keys[ctx->server_count] = strdup(key);
    ctx->server_values[ctx->server_count] = value ? strdup(value) : strdup("");
    ctx->server_count++;
}

void php_context_set_server(php_context* ctx, const char* key, const char* value) {
    php_context_add_server_var(ctx, key, value);
}

void php_context_set_get(php_context* ctx, const char* key, const char* value) {
    (void)ctx; (void)key; (void)value;
    // GET variables are handled via query string parsing in PHP
}

void php_context_set_post(php_context* ctx, const char* key, const char* value) {
    (void)ctx; (void)key; (void)value;
    // POST variables are handled via post data parsing in PHP
}

void php_context_set_cookie(php_context* ctx, const char* key, const char* value) {
    (void)ctx; (void)key; (void)value;
    // Cookies are handled via HTTP_COOKIE header
}

void php_context_set_env(php_context* ctx, const char* key, const char* value) {
    (void)ctx; (void)key; (void)value;
    // Environment variables set via $_ENV
}

void php_context_set_document_root(php_context* ctx, const char* root) {
    if (!ctx) return;
    if (ctx->document_root) free(ctx->document_root);
    ctx->document_root = root ? strdup(root) : NULL;
}

void php_context_set_script_filename(php_context* ctx, const char* filename) {
    if (!ctx) return;
    if (ctx->script_filename) free(ctx->script_filename);
    ctx->script_filename = filename ? strdup(filename) : NULL;
}

void php_context_set_post_data(php_context* ctx, const char* data, size_t len) {
    if (!ctx) return;
    if (ctx->post_data) free(ctx->post_data);
    ctx->post_data = NULL;
    ctx->post_data_len = 0;
    ctx->post_data_read = 0;

    if (data && len > 0) {
        ctx->post_data = malloc(len);
        if (ctx->post_data) {
            memcpy(ctx->post_data, data, len);
            ctx->post_data_len = len;
        }
    }
}

void php_context_free(php_context* ctx) {
    if (!ctx) return;

    if (ctx->document_root) free(ctx->document_root);
    if (ctx->script_filename) free(ctx->script_filename);
    if (ctx->post_data) free(ctx->post_data);
    if (ctx->output_buffer) free(ctx->output_buffer);
    if (ctx->headers_buffer) free(ctx->headers_buffer);

    if (ctx->server_keys) {
        for (size_t i = 0; i < ctx->server_count; i++) {
            free(ctx->server_keys[i]);
        }
        free(ctx->server_keys);
    }
    if (ctx->server_values) {
        for (size_t i = 0; i < ctx->server_count; i++) {
            free(ctx->server_values[i]);
        }
        free(ctx->server_values);
    }

    free(ctx);
}

// PHP engine lifecycle (staged implementation)
// NOTE: This layer is currently compile-safe scaffolding for callback/context flow.
// Real Zend/SAPI lifecycle wiring (sapi_startup/php_module_startup/request lifecycle)
// is intentionally deferred until libphp embedding symbols and ABI integration are finalized.
int php_engine_startup(const char* version) {
    (void)version;
#ifdef PHP_EMBED_AVAILABLE
    static unsigned char initialized = 0;
    if (initialized) {
        return 0;
    }

    if (php_embed_init(0, NULL) == FAILURE) {
        return -1;
    }
    initialized = 1;
#endif
    return 0;
}

void php_engine_shutdown(void) {
#ifdef PHP_EMBED_AVAILABLE
    php_embed_shutdown();
#endif
}

php_response* php_execute(php_context* ctx, const char* script) {
    if (!ctx || !script) {
        return NULL;
    }

    // Set current context for this thread
    current_context = ctx;
    current_thread_index = ctx->thread_index;

    // Reset output buffer
    ctx->output_len = 0;
    ctx->headers_len = 0;
    ctx->http_status = 200;

    // Register server variables with Go
    for (size_t i = 0; i < ctx->server_count; i++) {
        go_register_variables(ctx->thread_index, ctx->server_keys[i], ctx->server_values[i]);
    }

    // Build response
    php_response* resp = calloc(1, sizeof(php_response));
    if (!resp) {
        current_context = NULL;
        return NULL;
    }

#ifdef PHP_EMBED_AVAILABLE
    zend_file_handle file_handle;
    memset(&file_handle, 0, sizeof(file_handle));
    file_handle.type = ZEND_HANDLE_FILENAME;
    file_handle.filename = script;

    int exec_result = php_execute_script(&file_handle);
    (void)exec_result;

    if (ctx->output_len > 0) {
        resp->body = malloc(ctx->output_len);
        if (resp->body) {
            memcpy(resp->body, ctx->output_buffer, ctx->output_len);
            resp->body_len = ctx->output_len;
        }
    }

    resp->status = ctx->http_status;
    if (resp->status == 0) {
        resp->status = 200;
    }

    const char* default_headers = "Content-Type: text/html; charset=utf-8\r\nX-Powered-By: Maboo";
    resp->headers = strdup(default_headers);
    resp->headers_len = strlen(resp->headers);
#else
    // Fallback when embed symbols are unavailable at compile time.
    const char* html_template =
        "<!DOCTYPE html>\n"
        "<html><head><title>Maboo PHP</title></head>\n"
        "<body><h1>Maboo Embedded PHP</h1>\n"
        "<p>Script: <code>%s</code></p>\n"
        "<p>Thread: %d</p>\n"
        "<p><em>PHP embedding unavailable in current build</em></p>\n"
        "</body></html>";

    size_t html_len = strlen(html_template) + strlen(script) + 32;
    resp->body = malloc(html_len);
    if (resp->body) {
        snprintf(resp->body, html_len, html_template, script, ctx->thread_index);
        resp->body_len = strlen(resp->body);
    }

    resp->status = 200;
    const char* default_headers = "Content-Type: text/html; charset=utf-8\r\nX-Powered-By: Maboo";
    resp->headers = strdup(default_headers);
    resp->headers_len = strlen(resp->headers);
#endif

    current_context = NULL;
    return resp;
}

void php_response_free(php_response* resp) {
    if (!resp) return;

    if (resp->headers) free(resp->headers);
    if (resp->body) free(resp->body);
    free(resp);
}
