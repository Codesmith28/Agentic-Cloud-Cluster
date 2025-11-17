import React, { useState } from 'react';
import {
  Box,
  Paper,
  Typography,
  TextField,
  Button,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Grid,
  Alert,
  Chip,
  Slider,
  FormHelperText,
} from '@mui/material';
import SendIcon from '@mui/icons-material/Send';
import { tasksAPI } from '../../api/tasks';
import { TASK_TAGS, TASK_TAG_LABELS, K_VALUE_OPTIONS, K_VALUE } from '../../utils/constants';

const SubmitTask = () => {
  const [formData, setFormData] = useState({
    docker_image: '',
    cpu_required: 1.0,
    memory_required: 2.0,
    storage_required: 5.0,
    gpu_required: 0.0,
    tag: '', // NEW FIELD
    k_value: K_VALUE.DEFAULT, // NEW FIELD
  });

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState(null);
  const [success, setSuccess] = useState(false);

  const handleChange = (field) => (event) => {
    setFormData({
      ...formData,
      [field]: event.target.value,
    });
    setError(null);
    setSuccess(false);
  };

  const handleKValueChange = (event, newValue) => {
    setFormData({
      ...formData,
      k_value: newValue,
    });
  };

  const validateForm = () => {
    if (!formData.docker_image.trim()) {
      setError('Docker image is required');
      return false;
    }
    if (!formData.tag) {
      setError('Task tag is required');
      return false;
    }
    if (formData.cpu_required <= 0) {
      setError('CPU must be greater than 0');
      return false;
    }
    if (formData.memory_required <= 0) {
      setError('Memory must be greater than 0');
      return false;
    }
    return true;
  };

  const handleSubmit = async (e) => {
    e.preventDefault();
    
    if (!validateForm()) return;

    setLoading(true);
    setError(null);
    setSuccess(false);

    try {
      const response = await tasksAPI.submitTask(formData);
      console.log('Task submitted:', response.data);
      setSuccess(true);
      
      // Reset form after successful submission
      setTimeout(() => {
        setFormData({
          docker_image: '',
          cpu_required: 1.0,
          memory_required: 2.0,
          storage_required: 5.0,
          gpu_required: 0.0,
          tag: '',
          k_value: K_VALUE.DEFAULT,
        });
        setSuccess(false);
      }, 3000);
    } catch (err) {
      setError(err.response?.data?.message || 'Failed to submit task');
    } finally {
      setLoading(false);
    }
  };

  return (
    <Paper elevation={3} sx={{ p: 4, maxWidth: 800, mx: 'auto' }}>
      <Typography variant="h5" gutterBottom>
        Submit New Task
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{ mb: 3 }}>
        Configure your Docker-based task with resource requirements and scheduling parameters
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {success && (
        <Alert severity="success" sx={{ mb: 2 }}>
          Task submitted successfully! It will be queued for execution.
        </Alert>
      )}

      <Box component="form" onSubmit={handleSubmit}>
        <Grid container spacing={3}>
          {/* Docker Image */}
          <Grid item xs={12}>
            <TextField
              fullWidth
              required
              label="Docker Image"
              value={formData.docker_image}
              onChange={handleChange('docker_image')}
              placeholder="e.g., python:3.9, nvidia/cuda:11.8-base"
              helperText="The Docker image to run your task"
            />
          </Grid>

          {/* Task Tag - NEW FEATURE */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth required>
              <InputLabel>Task Tag</InputLabel>
              <Select
                value={formData.tag}
                onChange={handleChange('tag')}
                label="Task Tag"
              >
                {Object.values(TASK_TAGS).map((tag) => (
                  <MenuItem key={tag} value={tag}>
                    <Chip
                      label={TASK_TAG_LABELS[tag]}
                      size="small"
                      sx={{
                        bgcolor: getTagColor(tag),
                        color: 'white',
                        mr: 1,
                      }}
                    />
                  </MenuItem>
                ))}
              </Select>
              <FormHelperText>
                Select the workload type for scheduling optimization
              </FormHelperText>
            </FormControl>
          </Grid>

          {/* K-Value - NEW FEATURE */}
          <Grid item xs={12} md={6}>
            <FormControl fullWidth>
              <Typography gutterBottom>
                K-Value: <strong>{formData.k_value.toFixed(1)}</strong>
              </Typography>
              <Slider
                value={formData.k_value}
                onChange={handleKValueChange}
                min={K_VALUE.MIN}
                max={K_VALUE.MAX}
                step={K_VALUE.STEP}
                marks={[
                  { value: 1.5, label: '1.5' },
                  { value: 2.0, label: '2.0' },
                  { value: 2.5, label: '2.5' },
                ]}
                valueLabelDisplay="auto"
              />
              <FormHelperText>
                Scheduling priority multiplier (1.5 = low, 2.5 = high)
              </FormHelperText>
            </FormControl>
          </Grid>

          {/* CPU Required */}
          <Grid item xs={12} sm={6}>
            <TextField
              fullWidth
              required
              type="number"
              label="CPU Required (cores)"
              value={formData.cpu_required}
              onChange={handleChange('cpu_required')}
              inputProps={{ min: 0.1, max: 64, step: 0.1 }}
              helperText="Number of CPU cores needed"
            />
          </Grid>

          {/* Memory Required */}
          <Grid item xs={12} sm={6}>
            <TextField
              fullWidth
              required
              type="number"
              label="Memory Required (GB)"
              value={formData.memory_required}
              onChange={handleChange('memory_required')}
              inputProps={{ min: 0.5, max: 256, step: 0.5 }}
              helperText="Amount of RAM needed"
            />
          </Grid>

          {/* Storage Required */}
          <Grid item xs={12} sm={6}>
            <TextField
              fullWidth
              type="number"
              label="Storage Required (GB)"
              value={formData.storage_required}
              onChange={handleChange('storage_required')}
              inputProps={{ min: 1, max: 1000, step: 1 }}
              helperText="Disk space needed"
            />
          </Grid>

          {/* GPU Required */}
          <Grid item xs={12} sm={6}>
            <TextField
              fullWidth
              type="number"
              label="GPU Required (units)"
              value={formData.gpu_required}
              onChange={handleChange('gpu_required')}
              inputProps={{ min: 0, max: 8, step: 0.5 }}
              helperText="Number of GPUs needed"
            />
          </Grid>

          {/* Submit Button */}
          <Grid item xs={12}>
            <Button
              type="submit"
              variant="contained"
              size="large"
              fullWidth
              disabled={loading}
              startIcon={<SendIcon />}
            >
              {loading ? 'Submitting...' : 'Submit Task'}
            </Button>
          </Grid>
        </Grid>
      </Box>
    </Paper>
  );
};

// Helper function to get tag colors
const getTagColor = (tag) => {
  const colors = {
    'cpu-light': '#4caf50',
    'cpu-heavy': '#ff9800',
    'memory-light': '#2196f3',
    'memory-heavy': '#f44336',
    'gpu-training': '#9c27b0',
    'mixed': '#607d8b',
  };
  return colors[tag] || '#9e9e9e';
};

export default SubmitTask;
