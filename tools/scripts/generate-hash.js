const bcrypt = require('bcrypt');
const password = 'Khaled@2008';

bcrypt.hash(password, 12, (err, hash) => {
  if (err) {
    console.error('Error generating hash:', err);
    process.exit(1);
  }
  console.log(hash);
});