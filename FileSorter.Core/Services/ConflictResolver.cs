using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public class ConflictResolver : IConflictResolver
{
    public async Task<ConflictResolutionResult> ResolveConflictAsync(
        string sourcePath, 
        string destinationPath, 
        ConflictHandling conflictHandling,
        CancellationToken cancellationToken = default)
    {
        throw new NotImplementedException();
    }
}