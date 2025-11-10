import { useEffect, useState } from 'react';
import { listModels, updateTrafficWeight, deactivateModel, type ModelVersion } from '@/api/models';
import toast from 'react-hot-toast';

export default function AdminModels() {
  const [models, setModels] = useState<ModelVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [updatingModel, setUpdatingModel] = useState<number | null>(null);

  const fetchModels = async () => {
    try {
      const response = await listModels();
      setModels(response.info || []);
    } catch (error) {
      console.error('Failed to fetch models:', error);
      toast.error('Failed to load models');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchModels();
    // Refresh every 10 seconds
    const interval = setInterval(fetchModels, 10000);
    return () => clearInterval(interval);
  }, []);

  const handleUpdateTraffic = async (modelId: number, newWeight: number) => {
    if (newWeight < 0 || newWeight > 100) {
      toast.error('Traffic weight must be between 0 and 100');
      return;
    }

    setUpdatingModel(modelId);
    try {
      await updateTrafficWeight(modelId, newWeight);
      toast.success(`Updated traffic weight to ${newWeight}%`);
      await fetchModels();
    } catch (error: any) {
      toast.error(error.message || 'Failed to update traffic weight');
    } finally {
      setUpdatingModel(null);
    }
  };

  const handleDeactivate = async (modelId: number) => {
    if (!confirm('Are you sure you want to deactivate this model?')) {
      return;
    }

    setUpdatingModel(modelId);
    try {
      await deactivateModel(modelId);
      toast.success('Model deactivated');
      await fetchModels();
    } catch (error: any) {
      toast.error(error.message || 'Failed to deactivate model');
    } finally {
      setUpdatingModel(null);
    }
  };

  const activeModels = models.filter(m => m.is_active);
  const inactiveModels = models.filter(m => !m.is_active);
  const totalActiveWeight = activeModels.reduce((sum, m) => sum + m.traffic_weight, 0);
  const baseModelWeight = 100 - totalActiveWeight;

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-gray-600">Loading models...</div>
      </div>
    );
  }

  return (
    <div className="max-w-6xl mx-auto p-6">
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-gray-900 mb-2">AI Model Management</h1>
        <p className="text-gray-600">Manage fine-tuned models and traffic distribution</p>
      </div>

      {/* Traffic Distribution Summary */}
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4 mb-6">
        <h2 className="text-sm font-semibold text-blue-900 mb-2">Traffic Distribution</h2>
        <div className="flex items-center gap-4">
          <div>
            <span className="text-2xl font-bold text-blue-700">{baseModelWeight}%</span>
            <span className="text-sm text-blue-600 ml-2">Base Model</span>
          </div>
          {activeModels.map(model => (
            <div key={model.ID}>
              <span className="text-2xl font-bold text-green-700">{model.traffic_weight}%</span>
              <span className="text-sm text-green-600 ml-2">{model.version_name}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Active Models */}
      <div className="mb-8">
        <h2 className="text-xl font-semibold text-gray-900 mb-4">Active Models</h2>
        {activeModels.length === 0 ? (
          <div className="text-gray-500 italic">No active fine-tuned models</div>
        ) : (
          <div className="space-y-4">
            {activeModels.map(model => (
              <ModelCard
                key={model.ID}
                model={model}
                onUpdateTraffic={handleUpdateTraffic}
                onDeactivate={handleDeactivate}
                isUpdating={updatingModel === model.ID}
              />
            ))}
          </div>
        )}
      </div>

      {/* Inactive Models */}
      {inactiveModels.length > 0 && (
        <div>
          <h2 className="text-xl font-semibold text-gray-900 mb-4">Inactive Models</h2>
          <div className="space-y-4">
            {inactiveModels.map(model => (
              <ModelCard
                key={model.ID}
                model={model}
                onUpdateTraffic={handleUpdateTraffic}
                onDeactivate={handleDeactivate}
                isUpdating={updatingModel === model.ID}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

interface ModelCardProps {
  model: ModelVersion;
  onUpdateTraffic: (modelId: number, weight: number) => void;
  onDeactivate: (modelId: number) => void;
  isUpdating: boolean;
}

function ModelCard({ model, onUpdateTraffic, onDeactivate, isUpdating }: ModelCardProps) {
  const [trafficInput, setTrafficInput] = useState(String(model.traffic_weight));

  useEffect(() => {
    setTrafficInput(String(model.traffic_weight));
  }, [model.traffic_weight]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const weight = parseInt(trafficInput, 10);
    if (!isNaN(weight)) {
      onUpdateTraffic(model.ID, weight);
    }
  };

  return (
    <div
      className={`border rounded-lg p-5 ${
        model.is_active ? 'border-green-300 bg-green-50' : 'border-gray-300 bg-gray-50'
      }`}
    >
      <div className="flex items-start justify-between mb-3">
        <div>
          <h3 className="text-lg font-semibold text-gray-900">{model.version_name}</h3>
          <p className="text-sm text-gray-600">Base: {model.base_model}</p>
        </div>
        <div className="flex items-center gap-2">
          {model.is_active && (
            <span className="px-3 py-1 bg-green-100 text-green-800 text-xs font-semibold rounded-full">
              ACTIVE
            </span>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 gap-4 text-sm mb-4">
        <div>
          <span className="text-gray-600">Training Samples:</span>
          <span className="ml-2 font-medium">{model.training_data_size || 'N/A'}</span>
        </div>
        <div>
          <span className="text-gray-600">Created:</span>
          <span className="ml-2 font-medium">
            {new Date(model.created_at).toLocaleDateString()}
          </span>
        </div>
        <div>
          <span className="text-gray-600">Traffic Weight:</span>
          <span className="ml-2 font-medium">{model.traffic_weight}%</span>
        </div>
        {model.activated_at && (
          <div>
            <span className="text-gray-600">Activated:</span>
            <span className="ml-2 font-medium">
              {new Date(model.activated_at).toLocaleDateString()}
            </span>
          </div>
        )}
      </div>

      {model.is_active && (
        <form onSubmit={handleSubmit} className="flex items-center gap-3">
          <label className="text-sm font-medium text-gray-700">Update Traffic:</label>
          <input
            type="number"
            min="0"
            max="100"
            value={trafficInput}
            onChange={(e) => setTrafficInput(e.target.value)}
            className="w-20 px-3 py-1 border border-gray-300 rounded text-sm"
            disabled={isUpdating}
          />
          <span className="text-sm text-gray-600">%</span>
          <button
            type="submit"
            disabled={isUpdating}
            className="px-4 py-1 bg-blue-600 text-white text-sm rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isUpdating ? 'Updating...' : 'Update'}
          </button>
          <button
            type="button"
            onClick={() => onDeactivate(model.ID)}
            disabled={isUpdating}
            className="px-4 py-1 bg-red-600 text-white text-sm rounded hover:bg-red-700 disabled:opacity-50 disabled:cursor-not-allowed ml-2"
          >
            Deactivate
          </button>
        </form>
      )}
    </div>
  );
}
