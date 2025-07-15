using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public interface IConflictResolver
{
    Task<ConflictResolutionResult> ResolveConflictAsync(
        string sourcePath, 
        string destinationPath, 
        ConflictHandling conflictHandling,
        CancellationToken cancellationToken = default);
}