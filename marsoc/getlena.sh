#!/bin/bash
# 
# Downloads all samples from Lena API as JSON.
#
curl --insecure --header "Authorization: Token $1" https://lena/seq_results/api/samples/ -o api.json

#node -e "console.log(JSON.stringify(JSON.parse(require('fs').readFileSync(process.argv[1])), null, 4));" api.json > nice.json