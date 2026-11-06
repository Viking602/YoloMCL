#include "biomenoise.h"
#include <stdio.h>

int main(void) {
    const uint64_t seeds[] = {0,1,42,123456789,UINT64_MAX,UINT64_C(0x8000000000000000),UINT64_C(0x7fffffffffffffff)};
    const int coords[][2] = {{0,0},{-1,-1},{1023,0},{1024,0},{1040,0},{-1041,-1},{1000,1000},{-1000,1000},{1000,-1000},{-1000,-1000},{1536,2048},{2048,1536},{-4096,8192},{8192,-4096},{-12345,6789},{65536,-32768},{131072,262144},{1048576,-1012345},{10000000,0},{0,-10000000},{10000000,10000000},{29900000,-29900000}};
    int comma=0;
    printf("[\n");
    for(int si=0;si<7;si++) {
        EndNoise en; setEndSeed(&en,MC_1_21,seeds[si]);
        for(int ci=0;ci<(int)(sizeof(coords)/sizeof(coords[0]));ci++) {
            int x=coords[ci][0],z=coords[ci][1],id;
            mapEndBiome(&en,&id,x>>4,z>>4,1,1);
            printf("%s{\"seed\":\"%" PRId64 "\",\"x\":%d,\"z\":%d,\"biome\":%d}",comma++?",\n":"",(int64_t)seeds[si],x,z,id);
        }
    }
    printf("\n]\n");
    return 0;
}
