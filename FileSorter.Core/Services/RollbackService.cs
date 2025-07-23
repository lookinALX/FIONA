using System.Diagnostics;
using FileSorter.Core.Models;
using FileSorter.Core.Helpers;

namespace FileSorter.Core.Services;

public sealed class RollbackService : IRollbackService
{
    private readonly List<FileOperationRecord> _operationHistory = new();
    private readonly object _lock = new();

    public void RecordOperation(FileOperationRecord record)
    {
        ArgumentNullException.ThrowIfNull(record);

        lock (_lock)
        {
            _operationHistory.Add(record);
        }
    }

    public async Task<RollbackResult> RollbackAsync(CancellationToken cancellationToken = default)
    {
        var stopwatch = Stopwatch.StartNew();
        var result = new RollbackResult();

        List<FileOperationRecord> operationsToRollback;
        lock (_lock)
        {
            operationsToRollback = new List<FileOperationRecord>(_operationHistory);
            result.TotalOperations = operationsToRollback.Count;
        }

        if (!operationsToRollback.Any())
        {
            Console.WriteLine("No operations to rollback.");
            result.Success = true;
            return result;
        }

        // Rollback in reverse order
        operationsToRollback.Reverse();

        using (var progressBar = new ProgressBar(operationsToRollback.Count, "Rolling back operations"))
        {
            var processedCount = 0;

            foreach (var operation in operationsToRollback)
            {
                if (cancellationToken.IsCancellationRequested)
                    break;

                try
                {
                    await RollbackSingleOperationAsync(operation, cancellationToken);
                    result.SuccessfulRollbacks++;
                }
                catch (Exception ex)
                {
                    result.FailedRollbacks++;
                    result.Errors.Add($"Failed to rollback {operation.SourcePath}: {ex.Message}");
                }

                processedCount++;
                progressBar.Update(processedCount);
            }

            progressBar.Complete();
        }

        // Clear history after successful rollback
        if (result.FailedRollbacks == 0)
        {
            ClearHistory();
        }

        stopwatch.Stop();
        result.Duration = stopwatch.Elapsed;
        result.Success = result.FailedRollbacks == 0;

        return result;
    }

    private static async Task RollbackSingleOperationAsync(FileOperationRecord operation,
        CancellationToken cancellationToken)
    {
        if (!operation.Success)
            return; // Nothing to rollback if operation wasn't successful

        switch (operation.OperationType)
        {
            case FileOperationType.Move:
                await RollbackMoveOperationAsync(operation, cancellationToken);
                break;

            case FileOperationType.Copy:
                await RollbackCopyOperationAsync(operation, cancellationToken);
                break;

            default:
                throw new NotSupportedException(
                    $"Rollback for operation type {operation.OperationType} is not supported");
        }
    }

    private static async Task RollbackMoveOperationAsync(FileOperationRecord operation,
        CancellationToken cancellationToken)
    {
        // For move operations, move the file back to original location
        if (File.Exists(operation.DestinationPath))
        {
            await Task.Run(() =>
            {
                // If there's a backup of original file, restore it first
                if (!string.IsNullOrEmpty(operation.BackupPath) && File.Exists(operation.BackupPath))
                {
                    File.Move(operation.BackupPath, operation.SourcePath);
                    File.Delete(operation.DestinationPath);
                }
                else
                {
                    File.Move(operation.DestinationPath, operation.SourcePath);
                }
            }, cancellationToken);
        }
    }

    private static async Task RollbackCopyOperationAsync(FileOperationRecord operation,
        CancellationToken cancellationToken)
    {
        // For copy operations, just delete the copied file
        if (File.Exists(operation.DestinationPath))
        {
            await Task.Run(() => File.Delete(operation.DestinationPath), cancellationToken);
        }

        // If we made a backup of original file, restore it
        if (!string.IsNullOrEmpty(operation.BackupPath) && File.Exists(operation.BackupPath))
        {
            await Task.Run(() => { File.Move(operation.BackupPath, operation.SourcePath); }, cancellationToken);
        }
    }

    public void ClearHistory()
    {
        lock (_lock)
        {
            _operationHistory.Clear();
        }
    }

    public IReadOnlyList<FileOperationRecord> GetOperationHistory()
    {
        lock (_lock)
        {
            return _operationHistory.AsReadOnly();
        }
    }
}