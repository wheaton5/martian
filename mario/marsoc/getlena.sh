#!/bin/bash
# 
# Downloads all samples from Lena API as JSON.
# Usage:
#     getlena.sh <LENA_AUTH_TOKEN>
#
curl --insecure --header "Authorization: Token $LENA_AUTH_TOKEN" https://$LENA_HOST/seq_results/api/samples/ -o temp.json
node -e "console.log(JSON.stringify(JSON.parse(require('fs').readFileSync(process.argv[1])), null, 4));" temp.json > lena.json
rm temp.json
