const mongoose = require('mongoose');

const ResourceSchema = new mongoose.Schema(
  {
    cpu: { type: Number, default: 1 },
    memory: { type: String, default: '512Mi' },
    storage: { type: String, default: '1Gi' },
    gpu: { type: Number, default: 0 }
  },
  { _id: false }
);

const LogSchema = new mongoose.Schema(
  {
    ts: { type: Date, default: Date.now },
    source: { type: String, enum: ['stdout', 'stderr', 'system'], default: 'stdout' },
    line: { type: String }
  },
  { _id: false }
);

const TaskSchema = new mongoose.Schema(
  {
    owner: { type: mongoose.Schema.Types.ObjectId, ref: 'User', required: true },
    name: { type: String },
    title: { type: String, required: true },
    description: { type: String },
    dockerImage: { type: String },
    command: { type: String },
    resources: { type: ResourceSchema, default: () => ({}) },
    status: { type: String, enum: ['pending', 'running', 'completed', 'failed', 'cancelled'], default: 'pending' },
    assignedWorker: { type: String, default: null },
    runtimeSeconds: { type: Number, default: 0 },
    logs: { type: [LogSchema], default: [] }
  },
  { timestamps: true }
);

module.exports = mongoose.model('Task', TaskSchema);
