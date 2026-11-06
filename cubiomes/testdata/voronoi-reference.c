// Build against cubiomes e61f90580cbdd883214a8054670dacae655e59c0.
// cc -std=c11 -O2 -fwrapv voronoi-reference.c layers.c noise.c biomes.c -lm -o voronoi-reference
#include "layers.h"
#include <inttypes.h>
#include <stdio.h>

int main(void) {
    const uint64_t seeds[] = {0,1,42,123456789,UINT64_MAX,UINT64_C(0x8000000000000000),UINT64_C(0x7fffffffffffffff)};
    uint64_t random = 123456789;
    int comma = 0;
    printf("[\n");
    for (int si=0; si<7; si++) {
        uint64_t sha = getVoronoiSHA(seeds[si]);
        for (int i=0; i<256; i++) {
            int x, y, z, qx, qy, qz;
            if (i<128) {
                x=(i%16)-8; y=(i/16)-4; z=(i%7)-3;
            } else {
                random=random*UINT64_C(6364136223846793005)+1;
                x=(int)((random>>32)%60000001)-30000000;
                random=random*UINT64_C(6364136223846793005)+1;
                z=(int)((random>>32)%60000001)-30000000;
                y=(int)(random%4096)-2048;
            }
            voronoiAccess3D(sha,x,y,z,&qx,&qy,&qz);
            printf("%s{\"seed\":\"%" PRId64 "\",\"sha\":\"%" PRIu64 "\",\"x\":%d,\"y\":%d,\"z\":%d,\"qx\":%d,\"qy\":%d,\"qz\":%d}",
                comma++?",\n":"",(int64_t)seeds[si],sha,x,y,z,qx,qy,qz);
        }
    }
    printf("\n]\n");
    return 0;
}
