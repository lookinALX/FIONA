using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public interface ISortingService
{
    Task<SortingResult> SortFilesAsync(string directoryPath, SortingProfile profile, CancellationToken cancellationToken = default);
}