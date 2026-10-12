/* Synthetic syscall canary only. This file is not a production runtime. */
#include <arpa/inet.h>
#include <errno.h>
#include <fcntl.h>
#include <signal.h>
#include <stdbool.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <sys/wait.h>
#include <unistd.h>

static bool readable(const char *root, const char *name) {
  char path[1024], byte = 0;
  if (snprintf(path, sizeof(path), "%s/%s", root, name) >= (int)sizeof(path)) return false;
  int fd = open(path, O_RDONLY);
  if (fd < 0) return false;
  bool result = read(fd, &byte, 1) == 1 && byte == 'f';
  close(fd);
  return result;
}
static bool workspace(const char *root) {
  char path[1024];
  if (!readable(root, "workspace/allowed")) return false;
  if (snprintf(path, sizeof(path), "%s/workspace/written", root) >= (int)sizeof(path)) return false;
  int fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, 0600);
  if (fd < 0) return false;
  bool result = write(fd, "f", 1) == 1;
  close(fd);
  return result;
}
static bool network(int port) {
  int fd = socket(AF_INET, SOCK_STREAM, 0);
  if (fd < 0) return false;
  struct sockaddr_in address = {0};
  address.sin_family = AF_INET;
  address.sin_port = htons((unsigned short)port);
  address.sin_addr.s_addr = htonl(INADDR_LOOPBACK);
  bool result = connect(fd, (struct sockaddr *)&address, sizeof(address)) == 0;
  close(fd);
  return result;
}
static bool ipc(const char *root) {
  int fd = socket(AF_UNIX, SOCK_STREAM, 0);
  if (fd < 0) return false;
  struct sockaddr_un address = {0};
  address.sun_family = AF_UNIX;
  if (snprintf(address.sun_path, sizeof(address.sun_path), "%s/ipc", root) >= (int)sizeof(address.sun_path)) { close(fd); return false; }
  bool result = connect(fd, (struct sockaddr *)&address, sizeof(address)) == 0;
  close(fd);
  return result;
}
static bool execute(void) {
  pid_t child = fork();
  if (child < 0) return false;
  if (child == 0) { execl("/usr/bin/true", "true", NULL); _exit(126); }
  int status = 0;
  return waitpid(child, &status, 0) == child && WIFEXITED(status) && WEXITSTATUS(status) == 0;
}
static const char *boolean(bool value) { return value ? "true" : "false"; }
int main(int argc, char **argv) {
  if (argc != 4) return 2;
  const char *root = argv[1];
  char byte = 0;
  bool descriptor = read(3, &byte, 1) == 1 && byte == 'f';
  const char *environment = getenv("FORT_SYNTHETIC_SECRET");
  printf("{\"workspace\":%s,\"owner\":%s,\"ledger\":%s,\"sibling\":%s,\"provider\":%s,\"home\":%s,\"symlink\":%s,\"environment\":%s,\"descriptor\":%s,\"process\":%s,\"ipc\":%s,\"network\":%s,\"exec\":%s}\n",
    boolean(workspace(root)), boolean(readable(root,"owner")), boolean(readable(root,"ledger")), boolean(readable(root,"sibling")), boolean(readable(root,"provider")), boolean(readable(root,"home/secret")), boolean(readable(root,"workspace/escape")), boolean(environment && strcmp(environment,"synthetic") == 0), boolean(descriptor), boolean(kill((pid_t)atoi(argv[3]),0) == 0), boolean(ipc(root)), boolean(network(atoi(argv[2]))), boolean(execute()));
  return 0;
}
