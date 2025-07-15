using FileSorter.Core.Services;

namespace Tests;

public class ConflictHandlingTests
{
    private string _testDirectory = string.Empty;
    private ConflictResolver _conflictResolver = new();

    [SetUp]
    public void Setup()
    {
        
    }

    [Test]
    public async Task ConflictResolver_Skip_Strategy_Should_Return_ShouldProceedFalse()
    {
        // Arrange
        
        // Act
        
        //Assert
        
    }
}