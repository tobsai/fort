/* Disposable offline experiment. Never linked into Fort or used as its launcher. */
#include <errno.h>
#include <fcntl.h>
#include <sandbox.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/un.h>
#include <unistd.h>

/* Diagnostic query only; no socket connection or Mach message is sent. */
extern int sandbox_check(pid_t pid, const char *operation, int filter, ...);

static void outcome(const char *name, int result, int error) {
  printf("%s result=%d errno=%d\n", name, result, error);
  fflush(stdout);
}

static int apply(const char *path) {
  FILE *file = fopen(path, "rb");
  if (!file) return 90;
  char profile[65536];
  size_t length = fread(profile, 1, sizeof(profile) - 1, file);
  if (ferror(file) || !feof(file)) { fclose(file); return 91; }
  fclose(file);
  profile[length] = '\0';
  char *error = NULL;
  if (sandbox_init(profile, 0, &error) != 0) {
    fprintf(stderr, "sandbox_init failed: %s\n", error ? error : "unknown");
    sandbox_free_error(error);
    return 92;
  }
  (void)write(STDOUT_FILENO, "sandbox_applied\n", 16);
  return 0;
}

static void read_case(const char *label, const char *name) {
  errno = 0;
  int fd = open(name, O_RDONLY);
  int error = errno;
  char byte = 0;
  int result = fd < 0 ? -1 : (int)read(fd, &byte, 1);
  if (fd >= 0) { error = errno; close(fd); }
  outcome(label, result, error);
}

int main(int argc, char **argv) {
  if (argc < 3) return 2;
  if (strcmp(argv[1], "guard") == 0) {
    if (argc < 4) return 2;
    if (sandbox_check(getpid(), "network-outbound", 0) != 0) return 95;
    int applied = apply(argv[2]);
    if (applied) return applied;
    int outbound = sandbox_check(getpid(), "network-outbound", 0);
    outcome("guard_network_query", outbound, 0);
    if (outbound != 1) return 93;
    (void)write(STDOUT_FILENO, "guard_exec_begin\n", 17);
    execv(argv[3], &argv[3]);
    outcome("guard_exec", -1, errno);
    return 94;
  }
  if (strcmp(argv[1], "probe") != 0 || argc != 4) return 2;
  if (chdir(argv[3]) != 0) return 3;
  int read_fd = open("ordinary", O_RDONLY);
  int write_fd = open("descriptor-write", O_WRONLY);
  int pair[2], pipes[2];
  if (read_fd < 0 || write_fd < 0 || socketpair(AF_UNIX, SOCK_STREAM, 0, pair) != 0 || pipe(pipes) != 0) return 4;
  int listener = socket(AF_UNIX, SOCK_STREAM, 0);
  struct sockaddr_un address = {0};
  address.sun_family = AF_UNIX;
  if (snprintf(address.sun_path, sizeof(address.sun_path), "%s/local.sock", argv[3]) >= (int)sizeof(address.sun_path)) return 5;
  if (listener < 0 || bind(listener, (struct sockaddr *)&address, sizeof(address)) || listen(listener, 1)) return 6;
  if (strcmp(argv[2], "control") != 0) {
    int applied = apply(argv[2]);
    if (applied) return applied;
  }
  read_case("ordinary", "ordinary");
  read_case("workspace", "workspace/inside");
  read_case("symlink", "workspace/escape");
  read_case("hardlink", "hardlink");
  read_case("unicode", "unicode-\xce\xbb");
  read_case("newline", "line\nbreak");
  errno = 0;
  int created = open("new-file", O_CREAT | O_WRONLY | O_EXCL, 0600);
  outcome("create", created < 0 ? -1 : 0, errno);
  if (created >= 0) close(created);
  errno = 0;
  int renamed = rename("rename-from", "rename-to");
  outcome("rename", renamed, errno);
  char byte = 0;
  errno = 0;
  int read_result = (int)read(read_fd, &byte, 1);
  outcome("preopened_read", read_result, errno);
  errno = 0;
  int write_result = (int)write(write_fd, "X", 1);
  outcome("preopened_write", write_result, errno);
  errno = 0;
  int pair_result = (int)write(pair[0], "I", 1);
  if (pair_result == 1) pair_result = (int)read(pair[1], &byte, 1);
  outcome("preopened_unix_pair", pair_result, errno);
  errno = 0;
  int pipe_result = (int)write(pipes[1], "P", 1);
  if (pipe_result == 1) pipe_result = (int)read(pipes[0], &byte, 1);
  outcome("preopened_pipe", pipe_result, errno);
  errno = 0;
  int client = socket(AF_UNIX, SOCK_STREAM, 0);
  int connected = client < 0 ? -1 : connect(client, (struct sockaddr *)&address, sizeof(address));
  outcome("unix_path_connect", connected, errno);
  if (client >= 0) close(client);
  errno = 0;
  outcome("network_outbound_query", sandbox_check(getpid(), "network-outbound", 0), errno);
  errno = 0;
  outcome("network_inbound_query", sandbox_check(getpid(), "network-inbound", 0), errno);
  errno = 0;
  outcome("mach_lookup_query", sandbox_check(getpid(), "mach-lookup", 0), errno);
  close(read_fd); close(write_fd); close(listener);
  close(pair[0]); close(pair[1]); close(pipes[0]); close(pipes[1]);
  return 0;
}
