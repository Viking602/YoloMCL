// Targeted 1.21.4/1.21.5+ cases from the pinned cubiomes PR #162 fork.
#include "biomenoise.h"
#include <stdio.h>

int main(void) {
    const uint64_t seeds[] = {0,42,UINT64_MAX};
    int comma=0;
    puts("[");
    for(int si=0;si<3;si++) {
        BiomeNoise old,now; initBiomeNoise(&old,MC_1_21_4); initBiomeNoise(&now,MC_1_21_5);
        setBiomeSeed(&old,seeds[si],0); setBiomeSeed(&now,seeds[si],0);
        NetherNoise nn; setNetherSeed(&nn,seeds[si]);
        int seen[2]={0,0};
        for(int x=-8192;x<8192 && !(seen[0]&&seen[1]);x+=64) {
            for(int z=-8192;z<8192 && !(seen[0]&&seen[1]);z+=64) {
                int a=sampleBiomeNoise(&old,NULL,x>>2,32,z>>2,NULL,0);
                if(a!=dark_forest && a!=pale_garden) continue;
                int b=sampleBiomeNoise(&now,NULL,x>>2,32,z>>2,NULL,0);
                if(b!=pale_garden || seen[a==pale_garden]) continue;
                seen[a==pale_garden]=1;
                for(int version=0;version<3;version++) {
                    int64_t np[6];
                    int biome=sampleBiomeNoise(version?&now:&old,np,x>>2,32,z>>2,NULL,0);
                    if(comma++) puts(",");
                    printf("{\"version\":\"%s\",\"seed\":\"%" PRId64 "\",\"x\":%d,\"y\":128,\"z\":%d,\"biome\":%d,\"climate\":[",version==0?"1.21.4":version==1?"1.21.5":"1.21.11",(int64_t)seeds[si],x,z,biome);
                    for(int k=0;k<6;k++) printf("%s%" PRId64,k?",":"",np[k]);
                    printf("],\"nether\":%d}",getNetherBiome(&nn,x>>2,32,z>>2,NULL));
                }
            }
        }
        if(!(seen[0]&&seen[1])) return 2;
    }
    puts("\n]");
    return 0;
}
