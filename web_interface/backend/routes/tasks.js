const express = require('express');
const mongoose = require('mongoose');
const Task = require('../models/Task');
const auth = require('../middleware/auth');
const { checkRole } = require('../middleware/roles');

const router = express.Router();

// List tasks - users see their tasks, admins can pass ?all=true to see all
router.get('/', auth, async (req, res) => {
  try {
    const { status, q, sort } = req.query;
    const filter = {};
    if (status) filter.status = status;
    if (q) {
      // search by name/title or id (only include _id if it's a valid ObjectId)
      const ors = [{ title: new RegExp(q, 'i') }, { name: new RegExp(q, 'i') }]
      if (mongoose.isValidObjectId(q)) ors.push({ _id: q })
      filter.$or = ors
    }
    if (!(req.user.role === 'admin' && req.query.all === 'true')) {
      filter.owner = req.user.id;
    }
    let query = Task.find(filter);
    if (sort) {
      // simple sort like createdAt:desc
      const [key, dir] = sort.split(':');
      query = query.sort({ [key]: dir === 'desc' ? -1 : 1 });
    } else {
      query = query.sort({ createdAt: -1 });
    }
    const tasks = await query.exec();
    res.json({ tasks });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Create task (submit)
router.post('/', auth, async (req, res) => {
  try {
    const payload = {
      owner: req.user.id,
      name: req.body.name,
      title: req.body.title || req.body.name || 'Untitled Task',
      description: req.body.description,
      dockerImage: req.body.dockerImage,
      command: req.body.command,
      resources: req.body.resources
    };
    const t = new Task(payload);
    await t.save();
    res.status(201).json({ task: t });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Get task details (owner or admin)
router.get('/:id', auth, async (req, res) => {
  try {
    if (!mongoose.isValidObjectId(req.params.id)) return res.status(400).json({ message: 'Invalid task id' })
    const task = await Task.findById(req.params.id).populate('owner', '-password');
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner._id.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    res.json({ task });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Get stored logs for completed or running task
router.get('/:id/logs', auth, async (req, res) => {
  try {
    if (!mongoose.isValidObjectId(req.params.id)) return res.status(400).json({ message: 'Invalid task id' })
    const task = await Task.findById(req.params.id);
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    res.json({ logs: task.logs || [] });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Retry a failed task (simple stub: set status to pending and clear logs)
router.post('/:id/retry', auth, async (req, res) => {
  try {
    if (!mongoose.isValidObjectId(req.params.id)) return res.status(400).json({ message: 'Invalid task id' })
    const task = await Task.findById(req.params.id);
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    task.status = 'pending';
    task.logs = [];
    await task.save();
    // In a real system we'd enqueue the task to a scheduler here
    res.json({ message: 'Retry enqueued', task });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Duplicate task - clone an existing task (owner or admin)
router.post('/:id/duplicate', auth, async (req, res) => {
  try {
    if (!mongoose.isValidObjectId(req.params.id)) return res.status(400).json({ message: 'Invalid task id' })
    const task = await Task.findById(req.params.id);
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    const payload = {
      owner: req.user.id,
      name: task.name,
      title: task.title,
      description: task.description,
      dockerImage: task.dockerImage,
      command: task.command,
      resources: task.resources,
      status: 'pending',
      assignedWorker: null,
      logs: []
    }
    const nt = new Task(payload)
    await nt.save()
    res.status(201).json({ message: 'Task duplicated', task: nt })
  } catch (err) {
    res.status(500).json({ message: err.message })
  }
})

// Update task (owner only)
router.put('/:id', auth, async (req, res) => {
  try {
    const task = await Task.findById(req.params.id);
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    task.title = req.body.title ?? task.title;
    task.description = req.body.description ?? task.description;
    if (req.body.resources) task.resources = req.body.resources;
    if (req.body.status) task.status = req.body.status;
    await task.save();
    res.json({ task });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

// Delete/cancel task (owner or admin)
router.delete('/:id', auth, async (req, res) => {
  try {
    const task = await Task.findById(req.params.id);
    if (!task) return res.status(404).json({ message: 'Not found' });
    if (task.owner.toString() !== req.user.id && req.user.role !== 'admin') return res.status(403).json({ message: 'Forbidden' });
    // For running tasks, you might set status to cancelled and signal worker; here we delete
    await task.deleteOne();
    res.json({ message: 'Deleted' });
  } catch (err) {
    res.status(500).json({ message: err.message });
  }
});

module.exports = router;
