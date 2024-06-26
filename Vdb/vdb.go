package Vdb

import (
	"VreeDB/Collection"
	"VreeDB/FileMapper"
	"VreeDB/Filter"
	"VreeDB/Logger"
	"VreeDB/Utils"
	"VreeDB/Vector"
	"fmt"
	"sort"
	"time"
)

// Vdb is the main struct of the VectorDatabase
type Vdb struct {
	Collections map[string]*Collection.Collection
	Mapper      *FileMapper.FileMapper
}

// DB is the global Vdb
var DB *Vdb

// init initializes the Vdb
func init() {
	DB = &Vdb{Mapper: FileMapper.Mapper}
	Logger.Log.Log("VectorDatabase initialized")
}

// InitFileMapper initializes the FileMapper
func (v *Vdb) InitFileMapper() {
	// Create a slice out of the collections map
	var collections []string
	for _, key := range v.Collections {
		collections = append(collections, key.Name)
	}
	FileMapper.Mapper.Start(collections)
}

// AddCollection creates a new Collection
func (v *Vdb) AddCollection(name string, vectorDimension int, distanceFunc string) error {
	// Check if collection allready exists
	if _, ok := v.Collections[name]; ok {
		return fmt.Errorf("Collection with name %s allready exists", name)
	}
	v.Collections[name] = Collection.NewCollection(name, vectorDimension, distanceFunc)
	// Add the collection to the FileMapper
	v.Mapper.AddCollection(name)
	// Write the Collection to the FS
	err := v.Collections[name].WriteConfig()
	if err != nil {
		return err
	}
	Logger.Log.Log("Collection " + name + " added")
	return nil
}

// DeleteCollection deletes a Collection
func (v *Vdb) DeleteCollection(name string) error {
	if _, ok := v.Collections[name]; !ok {
		return fmt.Errorf("Collection with name %s does not exist", name)
	}
	delete(v.Collections, name)
	// Delete the Collection from the FileMapper
	v.Mapper.DelCollection(name)
	Logger.Log.Log("Collection " + name + " deleted")
	return nil
}

// ListCollections returns a list of all collections names
func (v *Vdb) ListCollections() []string {
	var collections []string
	for key := range v.Collections {
		collections = append(collections, key)
	}
	return collections
}

// Search searches for the nearest neighbours of the given target vector
func (v *Vdb) Search(collectionName string, target *Vector.Vector, queue *Utils.HeapControl, maxDistancePercent float64,
	filter *[]Filter.Filter) []*Utils.ResultSet {
	v.Collections[collectionName].Mut.RLock()
	defer v.Collections[collectionName].Mut.RUnlock()

	// if the collection is empty we return an empty slice
	if v.Collections[collectionName].DiagonalLength == 0 {
		return []*Utils.ResultSet{}
	}

	// Start the Queue Thread
	queue.StartThreads()

	// Add 1 to the queue waitgroup
	queue.AddToWaitGroup()

	// Get the starting time
	t := time.Now()
	Utils.NewSearchUnit(v.Collections[collectionName].Nodes, target, queue, filter, v.Collections[collectionName].DistanceFunc,
		v.Collections[collectionName].DimensionDiff, 0.1)

	// Close the channel and wait for the Queue to finish
	queue.CloseChannel()
	queue.Wg.Wait()

	// Print the time it took
	Logger.Log.Log("Search took: " + time.Since(t).String())

	// Get the nodes from the queue
	data := queue.GetNodes()

	// If this collection uses euclid and we have a maxDistancePercent > 0 we need to filter the results
	if v.Collections[collectionName].DistanceFuncName == "euclid" && maxDistancePercent > 0 {
		// If a result is greater than maxDistancePercent * DiagonalLength we remove it
		for i := 0; i < len(data); i++ {
			if data[i].Distance > maxDistancePercent*v.Collections[collectionName].DiagonalLength {
				data = append(data[:i], data[i+1:]...)
				i--
			}
		}
	}

	// Create the ResultSet
	results := make([]*Utils.ResultSet, len(data))

	// Get the Payloads back from the Memory Map
	for i := 0; i < len(data); i++ {
		m, err := FileMapper.Mapper.ReadPayload(data[i].Node.Vector.PayloadStart, collectionName)
		if err != nil {
			Logger.Log.Log("Error reading payload: " + err.Error())
			continue
		}
		results[i] = &Utils.ResultSet{Payload: m, Distance: data[i].Distance}
	}

	// Sort the results by distance, smallest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	return results
}

func (v *Vdb) IndexSearch(collectionName string, target *Vector.Vector, queue *Utils.HeapControl, maxDistancePercent float64, filter *[]Filter.Filter,
	indexName string, indexValue any) []*Utils.ResultSet {
	v.Collections[collectionName].Mut.RLock()
	defer v.Collections[collectionName].Mut.RUnlock()

	// if the collection is empty we return an empty slice
	if v.Collections[collectionName].DiagonalLength == 0 {
		return []*Utils.ResultSet{}
	}

	// Start the Queue Thread
	queue.StartThreads()

	// Add 1 to the queue waitgroup
	queue.AddToWaitGroup()

	// Get the starting time
	t := time.Now()
	Utils.NewSearchUnit(v.Collections[collectionName].Indexes[indexName].Entries[indexValue], target, queue, filter, v.Collections[collectionName].DistanceFunc,
		v.Collections[collectionName].DimensionDiff, 0.1)

	// Close the channel and wait for the Queue to finish
	queue.CloseChannel()
	queue.Wg.Wait()

	// Print the time it took
	Logger.Log.Log("Search took: " + time.Since(t).String())

	// Get the nodes from the queue
	data := queue.GetNodes()

	// If this collection uses euclid and we have a maxDistancePercent > 0 we need to filter the results
	if v.Collections[collectionName].DistanceFuncName == "euclid" && maxDistancePercent > 0 {
		// If a result is greater than maxDistancePercent * DiagonalLength we remove it
		for i := 0; i < len(data); i++ {
			if data[i].Distance > maxDistancePercent*v.Collections[collectionName].DiagonalLength {
				data = append(data[:i], data[i+1:]...)
				i--
			}
		}
	}

	// Create the ResultSet
	results := make([]*Utils.ResultSet, len(data))

	// Get the Payloads back from the Memory Map
	for i := 0; i < len(data); i++ {
		m, err := FileMapper.Mapper.ReadPayload(data[i].Node.Vector.PayloadStart, collectionName)
		if err != nil {
			Logger.Log.Log("Error reading payload: " + err.Error())
			continue
		}
		results[i] = &Utils.ResultSet{Payload: m, Distance: data[i].Distance}
	}

	// Sort the results by distance, smallest first
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})

	return results
}
