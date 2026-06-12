const fs = require('fs');
const path = require('path');
const dir = 'd:/code/github/noyo/noyo/frontend/src/views';
const files = fs.readdirSync(dir).filter(f => f.endsWith('.vue'));
let fixed = 0;

for (const file of files) {
  const filePath = path.join(dir, file);
  let content = fs.readFileSync(filePath, 'utf8');
  if (content.includes('computed(') && !/import.*computed.*from ['"]vue['"]/.test(content)) {
    const match = content.match(/import\s+{([^}]+)}\s+from\s+['"]vue['"]/);
    if (match && !match[1].includes('computed')) {
       const replacement = match[0].replace('{', '{ computed,');
       content = content.replace(match[0], replacement);
    } else {
       content = content.replace(/<script setup>/, '<script setup>\nimport { computed } from \'vue\';');
    }
    fs.writeFileSync(filePath, content, 'utf8');
    fixed++;
    console.log('Fixed ' + file);
  }
}
console.log('Total fixed: ' + fixed);
