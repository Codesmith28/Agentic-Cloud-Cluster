require('dotenv').config();
const express = require('express');
const cors = require('cors');
const connectDB = require('./config/db');

const authRoutes = require('./routes/auth');
const taskRoutes = require('./routes/tasks');
const adminRoutes = require('./routes/admin');

const app = express();
app.use(cors());
app.use(express.json());

const PORT = process.env.PORT || 5000;

(async () => {
  await connectDB(process.env.MONGO_URI);
  app.use('/api/auth', authRoutes);
  app.use('/api/tasks', taskRoutes);
  app.use('/api/admin', adminRoutes);

  app.get('/', (req, res) => res.send('Auth backend running'));

  app.listen(PORT, () => console.log(`Server running on port ${PORT}`));
})();
