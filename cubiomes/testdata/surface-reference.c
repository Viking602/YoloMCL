/* MIT cubiomes e61f90580cbdd883214a8054670dacae655e59c0, generator.c.
 * Compile against the pinned public source; this is test-only, never shipped. */
#include "generator.h"
#include <stdio.h>
#include <inttypes.h>
int main(void) {
    const int versions[] = {MC_1_18, MC_1_20, MC_1_21_1};
    const char *names[] = {"1.18.2", "1.20.1", "1.21.1"};
    const int64_t seeds[] = {0, 42, -557181663597411692LL, INT64_MIN, INT64_MAX};
    const int coords[][2] = {{0,0}, {-1,-1}, {-324,512}, {7475000,-7475000}};
    puts("version,large,seed,x,z,height");
    for (int v=0; v<3; v++) for (int large=0; large<2; large++) for (int s=0; s<5; s++) {
        Generator g; setupGenerator(&g, versions[v], large ? LARGE_BIOMES : 0);
        applySeed(&g, DIM_OVERWORLD, (uint64_t)seeds[s]);
        for (int p=0; p<4; p++) {
            float heights[49];
            if (mapApproxHeight(heights, NULL, &g, NULL, coords[p][0], coords[p][1], 7, 7)) return 1;
            for (int z=0; z<7; z++) for (int x=0; x<7; x++)
                printf("%s,%d,%" PRId64 ",%d,%d,%.9g\n", names[v], large, seeds[s], (coords[p][0]+x)*4, (coords[p][1]+z)*4, heights[z*7+x]);
        }
    }
    return 0;
}
