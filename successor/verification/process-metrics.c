#include <libproc.h>
#include <sys/resource.h>
#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>
#include <limits.h>

int main(int argc, char **argv) {
    if (argc != 2) return 2;
    char *end = NULL;
    long pid = strtol(argv[1], &end, 10);
    if (!end || *end || pid <= 0 || pid > INT_MAX) return 2;
    struct rusage_info_v4 info = {0};
    if (proc_pid_rusage((int)pid, RUSAGE_INFO_V4, (rusage_info_t *)&info) != 0) return 3;
    printf("{\"interruptWakeups\":%" PRIu64 ",\"packageIdleWakeups\":%" PRIu64 ",\"residentBytes\":%" PRIu64 ",\"physicalFootprintBytes\":%" PRIu64 "}\n",
           info.ri_interrupt_wkups, info.ri_pkg_idle_wkups, info.ri_resident_size, info.ri_phys_footprint);
    return 0;
}
