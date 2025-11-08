const express = require('express');
const User = require('../models/User');
const auth = require('../middleware/auth');
const { checkRole } = require('../middleware/roles');

const router = express.Router();

// Admin: list all users
router.get('/users', auth, checkRole('admin'), async (req, res) => {
  try {
    const users = await User.find().select('-password');
    res.json({ users });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Admin: delete user
router.delete('/users/:id', auth, checkRole('admin'), async (req, res) => {
  try {
    await User.findByIdAndDelete(req.params.id);
    res.json({ message: 'User deleted' });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

module.exports = router;
