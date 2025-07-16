using FileSorter.Core.Models;

namespace FileSorter.Core.Services;

public interface IRollbackService
{
    void RecordOperation(FileOperationRecord record);
    
    Task<RollbackResult> RollbackAsync(CancellationToken cancellationToken = default);
    
    void ClearHistory();
    
    IReadOnlyList<FileOperationRecord> GetOperationHistory();
}

