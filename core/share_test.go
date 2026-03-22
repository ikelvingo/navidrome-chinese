package core

import (
	"context"

	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Share", func() {
	var ds model.DataStore
	var share Share
	var mockedRepo rest.Persistable
	ctx := context.Background()

	BeforeEach(func() {
		ds = &tests.MockDataStore{}
		mockedRepo = ds.Share(ctx).(rest.Persistable)
		share = NewShare(ds)
	})

	Describe("NewRepository", func() {
		var repo rest.Persistable

		BeforeEach(func() {
			repo = share.NewRepository(ctx).(rest.Persistable)
			_ = ds.Album(ctx).Put(&model.Album{ID: "123", Name: "Album"})
		})

		Describe("Save", func() {
			It("it sets a random ID", func() {
				entity := &model.Share{Description: "test", ResourceIDs: "123"}
				id, err := repo.Save(entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(id).ToNot(BeEmpty())
				Expect(entity.ID).To(Equal(id))
			})

			It("does not truncate ASCII labels shorter than 30 characters", func() {
				_ = ds.MediaFile(ctx).Put(&model.MediaFile{ID: "456", Title: "Example Media File"})
				entity := &model.Share{Description: "test", ResourceIDs: "456"}
				_, err := repo.Save(entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(entity.Contents).To(Equal("Example Media File"))
			})

			It("truncates ASCII labels longer than 30 characters", func() {
				_ = ds.MediaFile(ctx).Put(&model.MediaFile{ID: "789", Title: "Example Media File But The Title Is Really Long For Testing Purposes"})
				entity := &model.Share{Description: "test", ResourceIDs: "789"}
				_, err := repo.Save(entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(entity.Contents).To(Equal("Example Media File But The ..."))
			})

			It("does not truncate CJK labels shorter than 30 runes", func() {
				_ = ds.MediaFile(ctx).Put(&model.MediaFile{ID: "456", Title: "éæ¥ã³ã³ãã¬ãã¯ï¿?})
				entity := &model.Share{Description: "test", ResourceIDs: "456"}
				_, err := repo.Save(entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(entity.Contents).To(Equal("éæ¥ã³ã³ãã¬ãã¯ï¿?))
			})

			It("truncates CJK labels longer than 30 runes", func() {
				_ = ds.MediaFile(ctx).Put(&model.MediaFile{ID: "789", Title: "ç§ã®ä¸­ã®å¹»æ³çä¸çè¦³åã³ãã®é¡ç¾ãæ³èµ·ãããããç¾å®ã§ã®åºæ¥äºã«é¢ããä¸èå¯"})
				entity := &model.Share{Description: "test", ResourceIDs: "789"}
				_, err := repo.Save(entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(entity.Contents).To(Equal("ç§ã®ä¸­ã®å¹»æ³çä¸çè¦³åã³ãã®é¡ç¾ãæ³èµ·ãããããç¾å®ï¿?.."))
			})
		})

		Describe("Update", func() {
			It("filters out read-only fields", func() {
				entity := &model.Share{}
				err := repo.Update("id", entity)
				Expect(err).ToNot(HaveOccurred())
				Expect(mockedRepo.(*tests.MockShareRepo).Cols).To(ConsistOf("description", "downloadable"))
			})
		})
	})
})
