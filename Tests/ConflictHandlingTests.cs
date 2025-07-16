using FileSorter.Core.Models;
using FileSorter.Core.Services;

namespace Tests;

public class ConflictHandlingTests
{
    private readonly string _fileDirectoryTest = @"F:\projects\FIONA\Tests\";
    private string _testDirectory = string.Empty;
    private ConflictResolver _conflictResolver = new();

    [SetUp]
    public void Setup()
    {
        _testDirectory = Path.Combine(_fileDirectoryTest, $"test_{Guid.NewGuid()}");
        Directory.CreateDirectory(_testDirectory);
    }
    
    [TearDown]
    public void TearDown()
    {
        if (Directory.Exists(_testDirectory))
        {
            Directory.Delete(_testDirectory, true);
        }
    }

    [Test]
    public async Task ConflictResolver_Skip_Strategy_Should_Return_ShouldProceedFalse()
    {
        // Arrange
        var sourceFile = Path.Combine(_testDirectory, "source.txt");
        var destinationFile = Path.Combine(_testDirectory, "destination.txt");
        
        File.WriteAllText(sourceFile, "source content");
        File.WriteAllText(destinationFile, "destination content");
        
        var conflictHandling = new ConflictHandling 
        { 
            Strategy = ConflictResolutionStrategy.Skip 
        };

        // Act
        var result = await _conflictResolver.ResolveConflictAsync(sourceFile, destinationFile, conflictHandling);

        // Assert
        Assert.That(result.ShouldProceed, Is.False);
        Assert.That(result.AppliedStrategy, Is.EqualTo(ConflictResolutionStrategy.Skip));
    }
    
    [Test]
    public async Task ConflictResolver_Rename_Strategy_Should_Generate_Unique_Name()
    {
        // Arrange
        var sourceFile = Path.Combine(_testDirectory, "source.txt");
        var destinationFile = Path.Combine(_testDirectory, "destination.txt");
        
        File.WriteAllText(sourceFile, "source content");
        File.WriteAllText(destinationFile, "destination content");
        
        var conflictHandling = new ConflictHandling 
        { 
            Strategy = ConflictResolutionStrategy.Rename 
        };

        // Act
        var result = await _conflictResolver.ResolveConflictAsync(sourceFile, destinationFile, conflictHandling);

        // Assert
        Assert.That(result.ShouldProceed, Is.True);
        Assert.That(result.AppliedStrategy, Is.EqualTo(ConflictResolutionStrategy.Rename));
        Assert.That(result.FinalDestinationPath, Is.Not.EqualTo(destinationFile));
        Assert.That(result.FinalDestinationPath, Does.Contain("destination_"));
    }
    
    [Test]
    public async Task ConflictResolver_No_Conflict_Should_Proceed_Normally()
    {
        // Arrange
        var sourceFile = Path.Combine(_testDirectory, "source.txt");
        var destinationFile = Path.Combine(_testDirectory, "nonexistent.txt");
        
        File.WriteAllText(sourceFile, "source content");
        
        var conflictHandling = new ConflictHandling 
        { 
            Strategy = ConflictResolutionStrategy.Rename 
        };

        // Act
        var result = await _conflictResolver.ResolveConflictAsync(sourceFile, destinationFile, conflictHandling);

        // Assert
        Assert.That(result.ShouldProceed, Is.True);
        Assert.That(result.FinalDestinationPath, Is.EqualTo(destinationFile));
    }
}

public class RollbackServiceTests
{
    private RollbackService _rollbackService = new();
    private string _testDirectory = string.Empty;

    [SetUp]
    public void Setup()
    {
        _testDirectory = Path.Combine(Path.GetTempPath(), $"test_{Guid.NewGuid()}");
        Directory.CreateDirectory(_testDirectory);
        _rollbackService = new RollbackService();
    }

    [TearDown]
    public void TearDown()
    {
        if (Directory.Exists(_testDirectory))
        {
            Directory.Delete(_testDirectory, true);
        }
    }

    [Test]
    public async Task RollbackService_Should_Rollback_Move_Operation()
    {
        // Arrange
        var sourceFile = Path.Combine(_testDirectory, "source.txt");
        var destinationFile = Path.Combine(_testDirectory, "destination.txt");
        
        File.WriteAllText(sourceFile, "test content");
        File.Move(sourceFile, destinationFile);
        
        var operation = new FileOperationRecord
        {
            SourcePath = sourceFile,
            DestinationPath = destinationFile,
            OperationType = FileOperationType.Move,
            Success = true,
            Timestamp = DateTime.Now
        };
        
        _rollbackService.RecordOperation(operation);

        // Act
        var result = await _rollbackService.RollbackAsync();

        // Assert
        Assert.That(result.Success, Is.True);
        Assert.That(result.SuccessfulRollbacks, Is.EqualTo(1));
        Assert.That(File.Exists(sourceFile), Is.True);
        Assert.That(File.Exists(destinationFile), Is.False);
    }

    [Test]
    public async Task RollbackService_Should_Rollback_Copy_Operation()
    {
        // Arrange
        var sourceFile = Path.Combine(_testDirectory, "source.txt");
        var destinationFile = Path.Combine(_testDirectory, "destination.txt");
        
        File.WriteAllText(sourceFile, "test content");
        File.Copy(sourceFile, destinationFile);
        
        var operation = new FileOperationRecord
        {
            SourcePath = sourceFile,
            DestinationPath = destinationFile,
            OperationType = FileOperationType.Copy,
            Success = true,
            Timestamp = DateTime.Now
        };
        
        _rollbackService.RecordOperation(operation);

        // Act
        var result = await _rollbackService.RollbackAsync();

        // Assert
        Assert.That(result.Success, Is.True);
        Assert.That(result.SuccessfulRollbacks, Is.EqualTo(1));
        Assert.That(File.Exists(sourceFile), Is.True);
        Assert.That(File.Exists(destinationFile), Is.False);
    }

    [Test]
    public void RollbackService_Should_Clear_History()
    {
        // Arrange
        var operation = new FileOperationRecord
        {
            SourcePath = "test",
            DestinationPath = "test",
            OperationType = FileOperationType.Copy,
            Success = true,
            Timestamp = DateTime.Now
        };
        
        _rollbackService.RecordOperation(operation);
        Assert.That(_rollbackService.GetOperationHistory().Count, Is.EqualTo(1));

        // Act
        _rollbackService.ClearHistory();

        // Assert
        Assert.That(_rollbackService.GetOperationHistory().Count, Is.EqualTo(0));
    }
}